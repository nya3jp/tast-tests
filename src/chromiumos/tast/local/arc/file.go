// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
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

// directWriteFile writes to a file in Android file system with android-sh.
func directWriteFile(ctx context.Context, filename string, data []byte) error {
	cmd := BootstrapCommand(ctx, "/system/bin/sh", "-c", "cat > \"$1\"", "-", filename)
	cmd.Stdin = bytes.NewBuffer(data)
	return cmd.Run()
}
