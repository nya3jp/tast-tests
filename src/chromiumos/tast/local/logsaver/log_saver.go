// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package logsaver provides the utilities to read the log files during a test
// run.
package logsaver

import (
	"context"
	"io"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// LogSaver keeps track of the position of a log file and saves a certain part
// of the log into a file.
//
// Typically, LogSaver will be used to split the log data into multiple parts.
// This will be helpful to split a longrunning and shared log data into parts
// per test case. See chrome.loggedInFixture for the usage.
//
// TODO(): Support log-rotated files if necessary.
// TODO(): Support non-file sources (e.g. dmesg command output).
type LogSaver struct {
	filename      string
	startPosition int64
	started       bool
	lastPosition  int64
}

// New creates a new LogSaver instance for the specified filename. When
// omitOldLog is true, the log in the file before this function is called will
// be omitted from the output.
func New(filename string, omitOldLogs bool) (*LogSaver, error) {
	var location int64
	if omitOldLogs {
		s, err := os.Stat(filename)
		if err != nil {
			return nil, err
		}
		location = s.Size()
	}
	return &LogSaver{
		filename:      filename,
		startPosition: location,
	}, nil
}

// Filename returns the name of the log file it observes.
func (ls *LogSaver) Filename() string {
	return ls.filename
}

// Start marks the current position of the log.
func (ls *LogSaver) Start(ctx context.Context) error {
	if ls.started {
		testing.ContextLog(ctx, "Start is already called")
	}
	s, err := os.Stat(ls.filename)
	if err != nil {
		return errors.Wrapf(err, "failed to get stat for %q", ls.filename)
	}
	ls.lastPosition = s.Size()
	ls.started = true
	return nil
}

// Started returns true if Start is invoked already but its corresponding
// StopAndSave is not yet called.
func (ls *LogSaver) Started() bool {
	return ls.started
}

func (ls *LogSaver) copyTo(position int64, filename string) error {
	fout, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open %q", filename)
	}
	defer fout.Close()
	fin, err := os.Open(ls.filename)
	if err != nil {
		return errors.Wrapf(err, "failed to open %q", ls.filename)
	}
	defer fin.Close()
	if position > 0 {
		if _, err := fin.Seek(position, 0); err != nil {
			return errors.Wrap(err, "failed to seek")
		}
	}
	_, err = io.Copy(fout, fin)
	return err
}

// StopAndSave checks the current position of the log, and saves the log from
// the last invocation of Start.
func (ls *LogSaver) StopAndSave(filename string) error {
	if !ls.started {
		return errors.New("Start is not invoked")
	}
	ls.started = false
	return ls.copyTo(ls.lastPosition, filename)
}

// SaveAll saves the entire content of the log file into a file. Note that
// the old log messages will be omitted when omitOldLog is true on creation.
func (ls *LogSaver) SaveAll(filename string) error {
	return ls.copyTo(ls.startPosition, filename)
}
