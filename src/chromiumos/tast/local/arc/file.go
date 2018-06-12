// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

// ReadFile reads a file in Android file system.
func ReadFile(filename string) ([]byte, error) {
	cmd := Command("cat", filename)
	return cmd.Output()
}

// WriteFile writes to a file in Android file system.
func WriteFile(filename string, data []byte) error {
	cmd := Command("sh", "-c", "cat > \"$1\"", "-", filename)
	w, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
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
