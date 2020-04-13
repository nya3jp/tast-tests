// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package linuxssh provides Linux specific operations conducted via SSH
// TODO(oka): simplify the code.
package linuxssh

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	cryptossh "golang.org/x/crypto/ssh"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// GetFile copies a file or directory from the host to the local machine.
// dst is the full destination name for the file or directory being copied, not
// a destination directory into which it will be copied. dst will be replaced
// if it already exists.
func GetFile(ctx context.Context, s *ssh.Conn, src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	// Create a temporary directory alongside the destination path.
	td, err := ioutil.TempDir(filepath.Dir(dst), filepath.Base(dst)+".")
	if err != nil {
		return errors.Wrap(err, "creating local temp dir failed")
	}
	defer os.RemoveAll(td)

	sb := filepath.Base(src)
	rcmd := s.Command("tar", "-c", "--gzip", "-C", filepath.Dir(src), sb)
	p, err := rcmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe")
	}
	if err := rcmd.Start(ctx); err != nil {
		return errors.Wrap(err, "running remote tar failed")
	}
	defer rcmd.Wait(ctx)
	defer rcmd.Abort()

	cmd := exec.CommandContext(ctx, "/bin/tar", "-x", "--gzip", "--no-same-owner", "-C", td)
	cmd.Stdin = p
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "running local tar failed")
	}

	if err := os.Rename(filepath.Join(td, sb), dst); err != nil {
		return errors.Wrap(err, "moving local file failed")
	}
	return nil
}

// SymlinkPolicy describes how symbolic links should be handled by PutFiles.
type SymlinkPolicy = int

const (
	// PreserveSymlinks indicates that symlinks should be preserved during the copy.
	PreserveSymlinks SymlinkPolicy = iota
	// DereferenceSymlinks indicates that symlinks should be dereferenced and turned into normal files.
	DereferenceSymlinks
)

// findChangedFiles returns a subset of files that differ between the local machine
// and the remote machine. This function is intended for use when pushing files to s;
// an error is returned if one or more files are missing locally, but not if they're
// only missing remotely. Local directories are always listed as having been changed.
func findChangedFiles(ctx context.Context, s *ssh.Conn, files map[string]string) (map[string]string, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// Sort local names.
	lp := make([]string, 0, len(files))
	for l := range files {
		lp = append(lp, l)
	}
	sort.Strings(lp)

	// TODO(derat): For large binary files, it may be faster to do an extra round trip first
	// to get file sizes. If they're different, there's no need to spend the time and
	// CPU to run sha1sum.
	rp := make([]string, len(lp))
	for i, l := range lp {
		rp[i] = files[l]
	}

	var lh, rh map[string]string
	ch := make(chan error, 2)
	go func() {
		var err error
		lh, err = getLocalSHA1s(lp)
		ch <- err
	}()
	go func() {
		var err error
		rh, err = getRemoteSHA1s(ctx, s, rp)
		ch <- err
	}()
	for i := 0; i < 2; i++ {
		if err := <-ch; err != nil {
			return nil, errors.Wrap(err, "failed to get SHA1(s)")
		}
	}

	cf := make(map[string]string)
	for i, l := range lp {
		r := rp[i]
		// TODO(derat): Also check modes, maybe.
		if lh[l] != rh[r] {
			cf[l] = r
		}
	}
	return cf, nil
}

// getRemoteSHA1s returns SHA1s for the files paths on s.
// Missing files are excluded from the returned map.
func getRemoteSHA1s(ctx context.Context, s *ssh.Conn, paths []string) (map[string]string, error) {
	out, err := s.Command("sha1sum", paths...).Output(ctx)
	if err != nil {
		// TODO(derat): Find a classier way to ignore missing files.
		if _, ok := err.(*cryptossh.ExitError); !ok {
			return nil, errors.Wrap(err, "failed to hash files")
		}
	}

	sums := make(map[string]string, len(paths))
	for _, l := range strings.Split(string(out), "\n") {
		if l == "" {
			continue
		}
		f := strings.Fields(l)
		if len(f) != 2 {
			return nil, errors.Errorf("unexpected line %q from sha1sum", l)
		}
		sums[f[1]] = f[0]
	}
	return sums, nil
}

// getLocalSHA1s returns SHA1s for files in paths.
// An error is returned if any files are missing.
func getLocalSHA1s(paths []string) (map[string]string, error) {
	sums := make(map[string]string, len(paths))

	for _, p := range paths {
		if fi, err := os.Stat(p); err != nil {
			return nil, err
		} else if fi.IsDir() {
			// Use a bogus hash for directories to ensure they're copied.
			sums[p] = "dir-hash"
			continue
		}

		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		h := sha1.New()
		if _, err := io.Copy(h, f); err != nil {
			return nil, err
		}
		sums[p] = hex.EncodeToString(h.Sum(nil))
	}

	return sums, nil
}

// tarTransformFlag returns a GNU tar --transform flag for renaming path s to d when
// creating an archive.
func tarTransformFlag(s, d string) string {
	esc := func(s string, bad []string) string {
		for _, b := range bad {
			s = strings.Replace(s, b, "\\"+b, -1)
		}
		return s
	}
	return fmt.Sprintf("--transform=s,^%s$,%s,",
		esc(regexp.QuoteMeta(s), []string{","}),
		esc(d, []string{"\\", ",", "&"}))
}

// countingReader is an io.Reader wrapper that counts the transferred bytes.
type countingReader struct {
	r     io.Reader
	bytes int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	c, err := r.r.Read(p)
	r.bytes += int64(c)
	return c, err
}

// PutFiles copies files on the local machine to the host. files describes
// a mapping from a local file path to a remote file path. For example, the call:
//
//	PutFiles(ctx, conn, map[string]string{"/src/from": "/dst/to"})
//
// will copy the local file or directory /src/from to /dst/to on the remote host.
// Local file paths can be absolute or relative. Remote file paths must be absolute.
// SHA1 hashes of remote files are checked in advance to send updated files only.
// bytes is the amount of data sent over the wire (possibly after compression).
func PutFiles(ctx context.Context, s *ssh.Conn, files map[string]string,
	symlinkPolicy SymlinkPolicy) (bytes int64, err error) {
	af := make(map[string]string)
	for src, dst := range files {
		if !filepath.IsAbs(src) {
			p, err := filepath.Abs(src)
			if err != nil {
				return 0, errors.Errorf("source path %q could not be resolved", src)
			}
			src = p
		}
		if !filepath.IsAbs(dst) {
			return 0, errors.Errorf("destination path %q should be absolute", dst)
		}
		af[src] = dst
	}

	// TODO(derat): When copying a small amount of data, it may be faster to avoid the extra
	// comparison round trip(s) and instead just copy unconditionally.
	cf, err := findChangedFiles(ctx, s, af)
	if err != nil {
		return 0, err
	}
	if len(cf) == 0 {
		return 0, nil
	}

	args := []string{"-c", "--gzip", "-C", "/"}
	if symlinkPolicy == DereferenceSymlinks {
		args = append(args, "--dereference")
	}
	for l, r := range cf {
		args = append(args, tarTransformFlag(strings.TrimPrefix(l, "/"), strings.TrimPrefix(r, "/")))
	}
	for l := range cf {
		args = append(args, strings.TrimPrefix(l, "/"))
	}
	cmd := exec.CommandContext(ctx, "/bin/tar", args...)
	p, err := cmd.StdoutPipe()
	if err != nil {
		return 0, errors.Wrap(err, "failed to open stdout pipe")
	}
	if err := cmd.Start(); err != nil {
		return 0, errors.Wrap(err, "running local tar failed")
	}
	defer cmd.Wait()
	defer syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)

	rcmd := s.Command("tar", "-x", "--gzip", "--no-same-owner", "--recursive-unlink", "-C", "/")
	cr := &countingReader{r: p}
	rcmd.Stdin = cr
	if err := rcmd.Run(ctx); err != nil {
		return 0, errors.Wrap(err, "remote tar failed")
	}
	return cr.bytes, nil
}
