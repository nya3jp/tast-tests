// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// AndroidTmpDirPath is the path of tmp directory in Android.
const AndroidTmpDirPath = "/data/local/tmp"

// PullFile copies a file in Android to Chrome OS with adb pull.
func (d *Device) PullFile(ctx context.Context, src, dst string) error {
	return d.Command(ctx, "pull", src, dst).Run(testexec.DumpLogOnError)
}

// PushFile copies a file in Chrome OS to Android with adb push.
func (d *Device) PushFile(ctx context.Context, src, dst string) error {
	return d.Command(ctx, "push", src, dst).Run(testexec.DumpLogOnError)
}

// PushFileToTmpDir copies a file in Chrome OS to Android temp directory.
// The destination path within the Android is returned.
func (d *Device) PushFileToTmpDir(ctx context.Context, src string) (string, error) {
	dst := filepath.Join(AndroidTmpDirPath, filepath.Base(src))
	if err := d.PushFile(ctx, src, dst); err != nil {
		d.ShellCommand(ctx, "rm", dst).Run(testexec.DumpLogOnError)
		return "", errors.Wrapf(err, "failed to adb push %v to %v", src, dst)
	}
	return dst, nil
}

// ReadFile reads a file in Android file system with adb pull.
func (d *Device) ReadFile(ctx context.Context, filename string) ([]byte, error) {
	f, err := ioutil.TempFile("", "adb")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())

	if err = f.Close(); err != nil {
		return nil, err
	}

	if err = d.PullFile(ctx, filename, f.Name()); err != nil {
		return nil, err
	}
	return ioutil.ReadFile(f.Name())
}

// WriteFile writes to a file in Android file system with adb push.
func (d *Device) WriteFile(ctx context.Context, filename string, data []byte) error {
	f, err := ioutil.TempFile("", "adb")
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	if err := f.Chmod(0600); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return d.PushFile(ctx, f.Name(), filename)
}

// FileSize returns the size of the specified file in bytes. Returns an error if the file does not exist.
// Note: In contrast to PkgFileSize, FileSize accesses files via adb commands.
func (d *Device) FileSize(ctx context.Context, filename string) (int64, error) {
	// `stat -c %s` measures the size of a file in bytes.
	statOutput, err := d.ShellCommand(ctx, "stat", "-c", "%s", filename).Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrapf(err, "could not determine size of file: %s", filename)
	}

	fileSize, err := strconv.ParseInt(strings.TrimSpace(string(statOutput)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse file size for %q; got: %q want: decimal number", filename, string(statOutput))
	}

	return fileSize, nil
}

// TempDir creates a temporary directory under AndroidTmpDirPath in Android,
// then returns its absolute path.
// It is caller's responsibility to remove all the contents in the directory
// after its use. One of the typical use cases will be as follows:
//
//   tmpdir, err := a.MktempDir(ctx)
//   if err != nil {
//     ... // error handling
//   }
//   defer a.RemoveAll(tmpdir)
//   ... // Main code using tmpdir.
func (d *Device) TempDir(ctx context.Context) (string, error) {
	out, err := d.ShellCommand(ctx, "mktemp", "-d", "-p", AndroidTmpDirPath).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RemoveAll removes all files and directories under the path in Android.
// The path must be abspath.
func (d *Device) RemoveAll(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) {
		return errors.Errorf("path (%q) needs to be absolute path", path)
	}
	return d.ShellCommand(ctx, "rm", "-rf", path).Run(testexec.DumpLogOnError)
}

// SHA256Sum returns the sha256sum of the specified file as a string.
func (d *Device) SHA256Sum(ctx context.Context, filename string) (string, error) {
	res, err := d.ShellCommand(ctx, "sha256sum", filename).Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to run sha256sum command for the target file")
	}
	return strings.Fields(string(res))[0], nil
}
