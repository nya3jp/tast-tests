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
func PullFile(ctx context.Context, src, dst string) error {
	return adbCommand(ctx, "pull", src, dst).Run()
}

// PushFile copies a file in Chrome OS to Android with adb push.
func PushFile(ctx context.Context, src, dst string) error {
	return adbCommand(ctx, "push", src, dst).Run()
}

// ReadFile reads a file in Android file system with adb pull.
func ReadFile(ctx context.Context, filename string) ([]byte, error) {
	f, err := ioutil.TempFile("", "adb")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err = PullFile(ctx, filename, f.Name()); err != nil {
		return nil, err
	}
	return ioutil.ReadFile(f.Name())
}

// WriteFile writes to a file in Android file system with adb push.
func WriteFile(ctx context.Context, filename string, data []byte) error {
	f, err := ioutil.TempFile("", "adb")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err = ioutil.WriteFile(f.Name(), data, 0600); err != nil {
		return err
	}

	return PushFile(ctx, f.Name(), filename)
}

// directWriteFile writes to a file in Android file system with android-sh.
func directWriteFile(ctx context.Context, filename string, data []byte) error {
	cmd := bootstrapCommand(ctx, "sh", "-c", "cat > \"$1\"", "-", filename)
	cmd.Stdin = bytes.NewBuffer(data)
	return cmd.Run()
}
