// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package disk

import (
	"io/ioutil"
	"os"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
)

// Fill creates a temporary file in a directory that fills the disk by
// allocating it, but without performing any actual IO to write the content.
func Fill(dir string, tofill uint64) (string, error) {
	file, err := ioutil.TempFile(dir, "fill.*.dat")
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temp file in %s", dir)
	}
	defer file.Close()

	// Allocate disk space without writing content
	if err := unix.Fallocate(int(file.Fd()), 0, 0, int64(tofill)); err != nil {
		return "", errors.Wrapf(err, "failed to allocate %v bytes in %s", tofill, file.Name())
	}

	return file.Name(), nil
}

// FillUntil reates a temporary file in a directory that fills the disk until
// less than remaining bytes are available.
func FillUntil(dir string, remaining uint64) (string, error) {
	freeSpace, err := FreeSpace(dir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read free space in %s", dir)
	}

	if freeSpace < remaining {
		return "", errors.Errorf("insufficient free space; %v < %v", freeSpace, remaining)
	}

	tofill := freeSpace - remaining

	return Fill(dir, tofill)
}

// Refiller maintains the temporary fill file for you, so you can easily
// (re-)adjust the disk space.
type Refiller struct {
	// dir is the directory to put the temporary file in.
	dir string
	// filePath is the path to the temporary file, or empty if there isn't one.
	filePath string
}

// NewRefiller creates a new refiller. `dir` is the directory to put the
// temporary file in.
//
// Example:
//
//	refiller := NewRefiller(...)
//	defer func() {
//	  if err := refiller.Close(); err != nil {
//	    s.Error(...)
//	  }
//	}()
//	refiller.RefillUntil(...)
//	...
//	refiller.RefillUntil(...)
//	...
func NewRefiller(dir string) Refiller {
	return Refiller{dir: dir, filePath: ""}
}

// Close removes the temporary file if there is one. You can call it as many
// times as you want, and you should also call it at the end of your tast to
// clean things up.
func (r *Refiller) Close() error {
	if r.filePath != "" {
		if err := os.Remove(r.filePath); err != nil {
			return errors.Wrapf(err, "Unable to remove the temporary file %v", r.filePath)
		}
		r.filePath = ""
	}
	return nil
}

// Refill removes the temporary file previously created, and then fill the
// disk. See `Fill()`.
func (r *Refiller) Refill(tofill uint64) error {
	return r.refillImpl(Fill, tofill)
}

// RefillUntil removes the temporary file previously created, and then fill the
// disk. See `FillUntil()`.
func (r *Refiller) RefillUntil(remaining uint64) error {
	return r.refillImpl(FillUntil, remaining)
}

func (r *Refiller) refillImpl(fillFunc func(string, uint64) (string, error), bytes uint64) error {
	if err := r.Close(); err != nil {
		return err
	}

	filePath, err := fillFunc(r.dir, bytes)
	if err != nil {
		return err
	}

	r.filePath = filePath
	return nil
}
