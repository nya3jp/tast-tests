// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
)

// PullFile copies a file in Android to Chrome OS with adb pull.
func (a *ARC) PullFile(ctx context.Context, src, dst string) error {
	return adbCommand(ctx, "pull", src, dst).Run()
}

// PushFile copies a file in Chrome OS to Android with adb push.
func (a *ARC) PushFile(ctx context.Context, src, dst string) error {
	return adbCommand(ctx, "push", src, dst).Run()
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
	cmd := BootstrapCommand(ctx, "sh", "-c", "cat > \"$1\"", "-", filename)
	cmd.Stdin = bytes.NewBuffer(data)
	return cmd.Run()
}
