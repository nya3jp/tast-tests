// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package logsaver provides the utilities to read the log files during a test
// run.
package logsaver

import (
	"io"
	"os"

	"chromiumos/tast/errors"
)

// Marker records a specific position of a log file, and provides the ability
// to store the certain range of the log file into a different file.
type Marker struct {
	filename string
	offset   int64
}

// NewMarker checks the current position of the specified filename and returns
// an instance of Marker for the file and the position.
func NewMarker(filename string) (*Marker, error) {
	s, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	return &Marker{filename: filename, offset: s.Size()}, nil
}

// NewMarkerNoOffset returns a new Marker instance which can store the
// entire log file.
func NewMarkerNoOffset(filename string) *Marker {
	return &Marker{filename: filename}
}

// Save saves the log entries from the offset to the end of the file.
func (m *Marker) Save(filename string) error {
	fout, err := os.Create(filename)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", filename)
	}
	defer fout.Close()
	fin, err := os.Open(m.filename)
	if err != nil {
		return errors.Wrapf(err, "failed to open %q", m.filename)
	}
	defer fin.Close()
	if m.offset > 0 {
		if _, err := fin.Seek(m.offset, io.SeekStart); err != nil {
			return errors.Wrap(err, "failed to seek")
		}
	}
	_, err = io.Copy(fout, fin)
	return err
}
