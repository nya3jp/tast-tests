// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cleanup

import (
	"io/ioutil"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/disk"
)

// Fill creates a temporary file in a directory that fills the disk by
// allocating it, but without performing any actual IO to write the content.
func Fill(path string, tofill uint64) (string, error) {
	file, err := ioutil.TempFile(path, "fill.*.dat")
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temp file in %s", path)
	}
	defer file.Close()

	// Allocate disk space without writing content
	if err := syscall.Fallocate(int(file.Fd()), 0, 0, int64(tofill)); err != nil {
		return "", errors.Wrapf(err, "failed to allocate %v bytes in %s", tofill, file.Name())
	}

	return file.Name(), nil
}

// FillUntil reates a temporary file in a directory that fills the disk until
// less than remaining bytes are available.
func FillUntil(path string, remaining uint64) (string, error) {
	freeSpace, err := disk.FreeSpace(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read free space in %s", path)
	}

	if freeSpace < remaining {
		return "", errors.Errorf("insufficient free space; %v < %v", freeSpace, remaining)
	}

	tofill := freeSpace - remaining

	return Fill(path, tofill)
}
