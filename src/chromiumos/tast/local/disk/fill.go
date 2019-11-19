// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package disk

import (
	"io/ioutil"
	"os"
	"syscall"

	"chromiumos/tast/errors"
)

// Fill creates a temporary file in a directory that fills the disk by
// allocating it, but without performing any actual IO to write the content.
func Fill(path string, tofill uint64) (*os.File, error) {
	file, err := ioutil.TempFile(path, "fill.*.dat")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create temp file in %s", path)
	}
	defer file.Close()

	fd := file.Fd()

	// Allocate disk space without writing content
	if err := syscall.Fallocate(int(fd), 0, 0, int64(tofill)); err != nil {
		return nil, errors.Wrapf(err, "failed to allocate %v bytes in %s", tofill, file.Name())
	}

	return file, nil
}

// FillUntil reates a temporary file in a directory that fills the disk until
// less than remaining bytes are available.
func FillUntil(path string, remaining uint64) (*os.File, error) {
	freeSpace, err := FreeSpace(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read free space in %s", path)
	}

	if freeSpace < remaining {
		return nil, errors.Errorf("insufficient free space; %v < %v", freeSpace, remaining)
	}

	tofill := freeSpace - remaining

	return Fill(path, tofill)
}
