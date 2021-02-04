// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package log provides the handling of Chrome's log file.
package log

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Splitter can split the chrome.log into the specified segment and save it
// into a file.
type Splitter struct {
	logFile  string
	position int64
}

// NewSplitter creates a new splitter instance.
func NewSplitter() *Splitter {
	return &Splitter{}
}

// RealPathForChromeLog returns the real path of the chrome log file which is
// currently recorded.
func RealPathForChromeLog() (string, error) {
	const chromeLogFile = "/var/log/chrome/chrome"

	filename, err := os.Readlink(chromeLogFile)
	if err != nil {
		return "", errors.Wrapf(err, "filed to read the link of %q", chromeLogFile)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(chromeLogFile), filename)), nil
}

// Start marks the current position in chrome.log.
func (s *Splitter) Start(ctx context.Context) error {
	filename, err := RealPathForChromeLog()
	if err != nil {
		return err
	}
	stat, err := os.Stat(filename)
	if err != nil {
		return errors.Wrapf(err, "failed to get the information of %q", filename)
	}
	s.logFile = filename
	s.position = stat.Size()
	return nil
}

// SaveLogIntoFile saves the chrome log from the last call of Start into a file.
func (s *Splitter) SaveLogIntoFile(ctx context.Context, filename string) error {
	logFile, err := RealPathForChromeLog()
	testing.ContextLogf(ctx, "filepath: %q", logFile)
	if err != nil {
		return err
	}
	if logFile != s.logFile {
		return errors.Errorf("log filename changed (got %q, want %q), probably chrome restarted", logFile, s.logFile)
	}
	data, err := ioutil.ReadFile(logFile)
	if err != nil {
		return errors.Wrap(err, "failed to read")
	}

	return ioutil.WriteFile(filename, data[s.position:], 0644)
}
