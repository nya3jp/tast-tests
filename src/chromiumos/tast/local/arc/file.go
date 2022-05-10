// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
)

// TestBinaryDirPath is the directory to store test binaries which run inside ARC.
const TestBinaryDirPath = "/usr/local/libexec/arc-binary-tests"

// PullFile copies a file in Android to ChromeOS with adb pull.
func (a *ARC) PullFile(ctx context.Context, src, dst string) error {
	return a.device.PullFile(ctx, src, dst)
}

// PushFile copies a file in ChromeOS to Android with adb push.
func (a *ARC) PushFile(ctx context.Context, src, dst string) error {
	return a.device.PushFile(ctx, src, dst)
}

// PushFileToTmpDir copies a file in ChromeOS to Android temp directory.
// The destination path within the ARC container is returned.
func (a *ARC) PushFileToTmpDir(ctx context.Context, src string) (string, error) {
	return a.device.PushFileToTmpDir(ctx, src)
}

// AndroidDataDir returns the ChromeOS path from which /data/ can be accessed (/home/root/${USER_HASH}/android-data).
func AndroidDataDir(ctx context.Context, user string) (string, error) {
	// Cryptohome dir for the current user.
	rootCryptDir, err := cryptohome.SystemPath(ctx, user)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get the cryptohome directory for user: %s", user)
	}

	// android-data dir under the cryptohome dir (/home/root/${USER_HASH}/android-data)
	return filepath.Join(rootCryptDir, "android-data"), nil
}

// PkgDataDir returns the ChromeOS path of the directory that contains user files of a given Android package (/home/root/${USER_HASH}/android-data/data/media/0/Android/data/${PKG}).
func PkgDataDir(ctx context.Context, user, pkg string) (string, error) {
	andrDataDir, err := AndroidDataDir(ctx, user)
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
	return a.device.ReadFile(ctx, filename)
}

// WriteFile writes to a file in Android file system with adb push.
func (a *ARC) WriteFile(ctx context.Context, filename string, data []byte) error {
	return a.device.WriteFile(ctx, filename, data)
}

// FileSize returns the size of the specified file in bytes. Returns an error if the file does not exist.
// Note: In contrast to PkgFileSize, FileSize accesses files via adb commands.
func (a *ARC) FileSize(ctx context.Context, filename string) (int64, error) {
	return a.device.FileSize(ctx, filename)
}

// PkgFileSize returns the size of a specified file that belongs to a specified Android package in bytes. Returns an error if the file does not exist.
func PkgFileSize(ctx context.Context, user, pkg, filename string) (int64, error) {
	pkgDir, err := PkgDataDir(ctx, user, pkg)
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
func (a *ARC) TempDir(ctx context.Context) (string, error) {
	return a.device.TempDir(ctx)
}

// RemoveAll removes all files and directories under the path in Android.
// The path must be abspath.
func (a *ARC) RemoveAll(ctx context.Context, path string) error {
	return a.device.RemoveAll(ctx, path)
}
