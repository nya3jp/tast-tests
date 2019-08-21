// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MountOBB,
		Desc: "Verifies mount-obb's fuse works",
		Contacts: []string{
			"hashimoto@chromium.org", // original author.
			"arc-storage@google.com",
			"hidehiko@chromium.org", // Tast port.
		},
		// TODO(hidehiko,nya): registration_test.go is too strict.
		// Actually, this does not actually need "chrome", and
		// should be done very quickly so default Timeout should work.
		// Added "chrome" and Timeout as a workaround, because
		// it is not blocker, but we should revisit here.
		SoftwareDeps: []string{"android", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

func MountOBB(ctx context.Context, s *testing.State) {
	largeData := bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz"), 12345)

	// createFile creates a file at path with given content.
	createFile := func(path string, content []byte) error {
		if err := ioutil.WriteFile(path, content, 0644); err != nil {
			return errors.Wrapf(err, "failed to create %s", path)
		}
		return nil
	}

	// setUpFiles creates files to be tested.
	// - removed${I}.txt: once created, then removed later.
	// - file${I}.txt: created with containing "${I}" as its contents.
	// - large_file.data: created with largeData defined above.
	// where ${I} will be 0..99 inclusively.
	setUpFiles := func(dir string) error {
		// Create "removed" files and "file" files.
		for i := 0; i < 100; i++ {
			content := []byte(strconv.Itoa(i))
			if err := createFile(filepath.Join(dir, fmt.Sprintf("removed%d.txt", i)), content); err != nil {
				return err
			}

			if err := createFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), content); err != nil {
				return err
			}
		}

		// Create a file with large content.
		if err := createFile(filepath.Join(dir, "large_file.data"), largeData); err != nil {
			return err
		}

		// Remove "removed" files.
		for i := 0; i < 100; i++ {
			path := filepath.Join(dir, fmt.Sprintf("removed%d.txt", i))
			if err := os.Remove(path); err != nil {
				return errors.Wrapf(err, "failed to remove %s", path)
			}
		}
		return nil
	}

	// verifyFiles returns true on success.
	verifyFiles := func(dir string) bool {
		success := true

		// Verify existing files have expected content.
		for i := 0; i < 100; i++ {
			path := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
			expect := []byte(strconv.Itoa(i))
			if data, err := ioutil.ReadFile(path); err != nil {
				s.Errorf("Failed to read %s: %v", path, err)
				success = false
			} else if !bytes.Equal(data, expect) {
				s.Errorf("Unexpected contents in %s: got %q; want %q", path, data, expect)
				success = false
			}
		}

		// Verify removed files do not exist.
		for i := 0; i < 100; i++ {
			path := filepath.Join(dir, fmt.Sprintf("removed%d.txt", i))
			if _, err := os.Stat(path); err == nil {
				s.Error("Unexpected file exists: ", path)
				success = false
			} else if !os.IsNotExist(err) {
				s.Errorf("Stat(%q) failed: %v", path, err)
				success = false
			}
		}

		// Verify the large file has expected content.
		path := filepath.Join(dir, "large_file.data")
		if data, err := ioutil.ReadFile(path); err != nil {
			s.Errorf("Failed to read %s: %v", path, err)
			success = false
		} else if !bytes.Equal(data, largeData) {
			// Because data and largeData is huge, do not print in error.
			s.Errorf("Large data mismatch for %s", path)
			success = false
		}

		return success
	}

	// unmountAllUnder unmounts all mount points under dir.
	unmountAllUnder := func(dir string) error {
		ms, err := sysutil.MountInfoForPID(sysutil.SelfPID)
		if err != nil {
			return err
		}
		var errs []error
		for _, m := range ms {
			if !strings.HasPrefix(m.MountPath, dir) {
				continue
			}
			if err := unix.Unmount(m.MountPath, unix.MNT_DETACH); err != nil {
				errs = append(errs, errors.Wrap(err, "failed to unmount: "+m.MountPath))
			}
		}

		if errs != nil {
			return errors.Errorf("failed to unmount: %v", errs)
		}

		return nil
	}

	type fatType string
	const (
		FAT12 fatType = "12"
		FAT16         = "16"
		FAT32         = "32"
	)

	setUpImage := func(variant fatType, tempdir, path string) (retErr error) {
		if err := testexec.CommandContext(ctx, "truncate", "--size=64M", path).Run(testexec.DumpLogOnError); err != nil {
			return err
		}

		if err := testexec.CommandContext(ctx, "mkfs.vfat", "-F", string(variant), path).Run(testexec.DumpLogOnError); err != nil {
			return err
		}

		mountPath := filepath.Join(tempdir, "setup_mount")
		if err := os.MkdirAll(mountPath, 0755); err != nil {
			return errors.Wrap(err, "failed to create a dir at "+mountPath)
		}
		if err := testexec.CommandContext(ctx, "mount", path, mountPath).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to mount vfat")
		}
		defer func() {
			if retErr != nil {
				unix.Unmount(mountPath, unix.MNT_DETACH)
			}
		}()

		// Put files in the root directory.
		if err := setUpFiles(mountPath); err != nil {
			return err
		}

		// Put files in a child directory.
		subdirPath := filepath.Join(mountPath, "foo", "bar")
		if err := os.MkdirAll(subdirPath, 0755); err != nil {
			return errors.Wrap(err, "failed to create dir at "+subdirPath)
		}
		if err := setUpFiles(subdirPath); err != nil {
			return err
		}

		// Call Unmount without MNT_DETACH, which should commit the content of file system cached in kernel.
		if err := unix.Unmount(mountPath, 0); err != nil {
			return errors.Wrap(err, "failed to unmount vfat")
		}

		return nil
	}

	runTest := func(variant fatType) {
		s.Log("Testing FAT", variant)

		tempdir, err := ioutil.TempDir("", "fat"+string(variant))
		if err != nil {
			s.Error("Failed to create tempdir: ", err)
			return
		}
		defer func() {
			if err := unmountAllUnder(tempdir); err != nil {
				s.Log("Failed to unmount: ", err)
			}
			os.RemoveAll(tempdir)
		}()

		fatFile := filepath.Join(tempdir, "img")
		if err := setUpImage(variant, tempdir, fatFile); err != nil {
			s.Errorf("Failed to create FAT%s image: %v", variant, err)
			return
		}

		mountPath := filepath.Join(tempdir, "mount")
		if err := os.MkdirAll(mountPath, 0755); err != nil {
			s.Error("Failed to create a tempdir: ", err)
			return
		}

		// TODO(crbug.com/980349): Remove verbose log when the root cause
		// is identified.
		cmd := testexec.CommandContext(ctx, "mount-obb", "--v=1", fatFile, mountPath, "0" /* root UID */, "0" /* root GID */)
		if err := cmd.Start(); err != nil {
			s.Error("Failed to start mount-obb: ", err)
			return
		}
		defer func() {
			cmd.Kill()
			cmd.Wait()
		}()
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			ms, err := sysutil.MountInfoForPID(0)
			if err != nil {
				return testing.PollBreak(err)
			}
			for _, m := range ms {
				if m.Fstype == "fuse.mount-obb" && strings.HasPrefix(m.MountPath, tempdir) {
					return nil
				}
			}
			return errors.New("mount point is not yet ready")
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Error("Mount point did not become available: ", err)
			return
		}

		if !verifyFiles(mountPath) || !verifyFiles(filepath.Join(mountPath, "foo", "bar")) {
			// To diagnosing, dump the file list under "mountPath" to a file.
			if f, err := os.Create(filepath.Join(s.OutDir(), "filelist"+string(variant))); err != nil {
				s.Error("Failed to create log file: ", err)
			} else {
				defer f.Close()
				dumpCmd := testexec.CommandContext(ctx, "find", "-H", mountPath)
				dumpCmd.Stdout = f
				if err := dumpCmd.Run(testexec.DumpLogOnError); err != nil {
					s.Error("Failed to list files: ", err)
				}
			}
		}
	}

	runTest(FAT12)
	runTest(FAT16)
	runTest(FAT32)
}
