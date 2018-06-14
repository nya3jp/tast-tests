// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"io/ioutil"
	"os"
)

// PullFile copies a file in Android to Chrome OS with adb pull.
func PullFile(src, dst string) error {
	return adbCommand("pull", src, dst).Run()
}

// PushFile copies a file in Chrome OS to Android with adb push.
func PushFile(src, dst string) error {
	return adbCommand("push", src, dst).Run()
}

// ReadFile reads a file in Android file system with adb pull.
func ReadFile(filename string) ([]byte, error) {
	f, err := ioutil.TempFile("", "adb")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err = PullFile(filename, f.Name()); err != nil {
		return nil, err
	}
	return ioutil.ReadFile(f.Name())
}

// WriteFile writes to a file in Android file system with adb push.
func WriteFile(filename string, data []byte) error {
	f, err := ioutil.TempFile("", "adb")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err = ioutil.WriteFile(f.Name(), data, 0600); err != nil {
		return err
	}

	return PushFile(f.Name(), filename)
}

// directWriteFile writes to a file in Android file system with android-sh.
func directWriteFile(filename string, data []byte) error {
	cmd := bootstrapCommand("sh", "-c", "cat > \"$1\"", "-", filename)
	w, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		// Docs guarantee pipes to be closed only when Wait() is called.
		w.Close()
		return err
	}

	_, err = w.Write(data)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if werr := cmd.Wait(); err == nil {
		err = werr
	}
	return err
}
