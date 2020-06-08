// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
)

const (
	// ARCTmpDirPath is the path of tmp directory in ARC container.
	ARCTmpDirPath = "/data/local/tmp"

	// TestBinaryDirPath is the directory to store test binaries which run inside ARC container.
	TestBinaryDirPath = "/usr/local/libexec/arc-binary-tests"
)

// PullFile copies a file in Android to Chrome OS with adb pull.
func (a *ARC) PullFile(ctx context.Context, src, dst string) error {
	return adbCommand(ctx, "pull", src, dst).Run()
}

// PushFile copies a file in Chrome OS to Android with adb push.
func (a *ARC) PushFile(ctx context.Context, src, dst string) error {
	return adbCommand(ctx, "push", src, dst).Run()
}

// PushFileToTmpDir copies a file in Chrome OS to Android temp directory.
// The destination path within the ARC container is returned.
func (a *ARC) PushFileToTmpDir(ctx context.Context, src string) (string, error) {
	dst := filepath.Join(ARCTmpDirPath, filepath.Base(src))
	if err := a.PushFile(ctx, src, dst); err != nil {
		a.Command(ctx, "rm", dst).Run()
		return "", errors.Wrapf(err, "failed to adb push %v to %v", src, dst)
	}
	return dst, nil
}

// PushTestBinaryToTmpDir copies a series of test binary files in Chrome OS to Android temp directory.
// The format of the binary file name is: "<execName>_<abi>".
// For example, "footest_amd64", "footest_x86"
// The list of destination path of test binary files within the ARC container is returned.
func (a *ARC) PushTestBinaryToTmpDir(ctx context.Context, execName string) ([]string, error) {
	var execs []string
	for _, abi := range []string{"amd64", "x86", "arm"} {
		exec := filepath.Join(TestBinaryDirPath, execName+"_"+abi)
		if _, err := os.Stat(exec); err == nil {
			arcExec, err := a.PushFileToTmpDir(ctx, exec)
			if err != nil {
				a.Command(ctx, "rm", execs...).Run()
				return nil, err
			}
			execs = append(execs, arcExec)
		}
	}
	return execs, nil
}

// AndroidDataDir returns the ChromeOS path from which /data/ can be accessed (/home/root/${USER_HASH}/android-data).
func AndroidDataDir(user string) (string, error) {
	// Cryptohome dir for the current user.
	rootCryptDir, err := cryptohome.SystemPath(user)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get the cryptohome directory for user: %s", user)
	}

	// android-data dir under the cryptohome dir (/home/root/${USER_HASH}/android-data)
	return filepath.Join(rootCryptDir, "android-data"), nil
}

// PkgDataDir returns the ChromeOS path of the directory that contains user files of a given Android package (/home/root/${USER_HASH}/android-data/data/media/0/Android/data/${PKG}).
func PkgDataDir(user, pkg string) (string, error) {
	andrDataDir, err := AndroidDataDir(user)
	if err != nil {
		return "", errors.Wrap(err, "failed to get android-data path")
	}

	dataDir := filepath.Join(andrDataDir, "data/media/0/Android/data")
	if _, err := os.Stat(dataDir); err != nil {
		return "", errors.Wrapf(err, "cannot access Android data directory: %s", dataDir)
	}

	return filepath.Join(dataDir, pkg), nil
}

// ReadFile reads a file in Android file system with adb pull.
func (a *ARC) ReadFile(ctx context.Context, filename string) ([]byte, error) {
	f, err := ioutil.TempFile("", "adb")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())

	if err = f.Close(); err != nil {
		return nil, err
	}

	if err = a.PullFile(ctx, filename, f.Name()); err != nil {
		return nil, err
	}
	return ioutil.ReadFile(f.Name())
}

// WriteFile writes to a file in Android file system with adb push.
func (a *ARC) WriteFile(ctx context.Context, filename string, data []byte) error {
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

	return a.PushFile(ctx, f.Name(), filename)
}

// FileSize returns the size of the specified file in bytes. Returns an error if the file does not exist.
// Note: In contrast to PkgFileSize, FileSize accesses files via adb commands.
func (a *ARC) FileSize(ctx context.Context, filename string) (int64, error) {
	// `stat -c %s` measures the size of a file in bytes.
	statOutput, err := a.Command(ctx, "stat", "-c", "%s", filename).Output()
	if err != nil {
		return 0, errors.Wrapf(err, "could not determine size of file: %s", filename)
	}

	fileSize, err := strconv.ParseInt(strings.TrimSpace(string(statOutput)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "tried to check file size of %q but received unexpected result: %q", filename, string(statOutput))
	}

	return fileSize, nil
}

// PkgFileSize returns the size of a specified file that belongs to a specified Android package in bytes. Returns an error if the file does not exist.
func PkgFileSize(ctx context.Context, user, pkg, filename string) (int64, error) {
	pkgDir, err := PkgDataDir(user, pkg)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get package directory for %s", pkg)
	}

	fullPath := filepath.Join(pkgDir, filename)
	info, err := os.Stat(fullPath)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to access file: %s", fullPath)
	}
	return info.Size(), nil
}

// TempDir creates a temporary directory under ARCTmpDirPath in Android,
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
func (a *ARC) TempDir(ctx context.Context) (string, error) {
	out, err := a.Command(ctx, "mktemp", "-d", "-p", ARCTmpDirPath).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RemoveAll removes all files and directories under the path in Android.
// The path must be abspath.
func (a *ARC) RemoveAll(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) {
		return errors.Errorf("path (%q) needs to be absolute path", path)
	}
	return a.Command(ctx, "rm", "-rf", path).Run(testexec.DumpLogOnError)
}
