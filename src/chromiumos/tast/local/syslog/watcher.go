// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package syslog contains utilities for looking for specific messages in
// /var/log/messages and similar system log files.
package syslog

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/logs"
)

// Watcher allows a test to watch for a specific message in the system logs. It
// will watch both a old-style text log and the binary journald for the message.
// Unlike just running "grep loggname" or "journalctl | grep", Watcher will only
// report messages written after the test is started. It also deals with system
// text log rotation.
type Watcher struct {
	originalName  string        // The filename passed to NewWatcher.
	file          *os.File      // The currently open file.
	reader        *bufio.Reader // A Reader wrapping file.
	journalCursor string        // The binary journald cursor from logs.GetJournaldCursor
}

// NewWatcher returns a Watcher set to the current point in the file & binary
// system journald. The next call to HasMessage will start looking at the current
// point in the file & system journald.
func NewWatcher(ctx context.Context, filename string) (*Watcher, error) {
	journalCursor, err := logs.GetJournaldCursor(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting journald position")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "error opening log file")
	}
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, errors.Wrap(err, "error seeking to end of log file")
	}
	reader := bufio.NewReader(file)
	return &Watcher{originalName: filename, file: file, reader: reader, journalCursor: journalCursor}, nil
}

// Close closes the Watcher.
func (w *Watcher) Close() error {
	return w.file.Close()
}

// handleLogRotation should be called when reading hits EOF. It checks to see if
// the current log has been rotated (that is, if /var/log/messages has been
// moved to /var/log/messages.1 and a new /var/log/messages created). If it has,
// the Watcher is pointed at the new instance, and the caller is told to keep
// reading.
func (w *Watcher) handleLogRotation() (keepReading bool, err error) {
	stat, err := w.file.Stat()
	if err != nil {
		return false, errors.Wrap(err, "error stat'ing existing file")
	}
	origStat, err := os.Stat(w.originalName)
	if err != nil {
		if os.IsNotExist(err) {
			// Old log file was moved, but new file has not yet been created. Next
			// call to HasMessage() will come back in here and try again to open the
			// new file.
			return false, nil
		}
		return false, errors.Wrap(err, "error stat'ing original file")
	}

	if os.SameFile(stat, origStat) {
		// Just a normal EOF. Nothing more to read.
		return false, nil
	}
	// File was rotated; open new file. We don't handle the case where a
	// log file went through multiple rotations during a single test (that is,
	// we don't handle having /var/log/messages moved all the way to
	// /var/log/messages.2 in between two HasMessage() calls).
	file, err := os.Open(w.originalName)
	if err != nil {
		return false, errors.Wrap(err, "error opening new log file instance")
	}

	w.file.Close()
	w.file = file
	w.reader = bufio.NewReader(file)
	return true, nil
}

// hasMessageInFile searchs for the message in the plain-text log.
func (w *Watcher) hasMessageInFile(text string) (bool, error) {
	found := false
	for {
		line, err := w.reader.ReadString('\n')
		if strings.Index(line, text) != -1 {
			found = true
			// Do not return here. The next call to HasMessage should start at the
			// current end of the log file.
		}

		if err == io.EOF {
			keepReading, err := w.handleLogRotation()
			if err != nil {
				return found, errors.Wrap(err, "error handling log rotation")
			}
			if !keepReading {
				return found, nil
			}
		} else if err != nil {
			return found, errors.Wrap(err, "error reading log line")
		}
	}
}

// hasMessageInJournald searchs for the message in the binary journald log.
func (w *Watcher) hasMessageInJournald(ctx context.Context, text string) (bool, error) {
	// Update the cursor before searching. In theory, this can lead to two
	// successive HasMessages returning true when only a single message was logged
	// (if the message is logged between the time we get the cursor and the time
	// we do the search). However, this is better than the alternative, which is
	// to get the new cursor after searching, which risks missing a message
	// altogether. Since this is often called in a polling loop waiting for a
	// sub-process to log a "success" message, the first type of race is less
	// harmful.
	newCursor, err := logs.GetJournaldCursor(ctx)
	if err != nil {
		return false, errors.Wrap(err, "error getting journald position")
	}
	defer func() { w.journalCursor = newCursor }()

	reader, writer := io.Pipe()
	defer reader.Close()
	errs := make(chan error)
	go func() {
		err = logs.WriteJournaldLogs(ctx, writer, w.journalCursor, logs.JournaldCompact)
		writer.Close()
		if err != nil {
			errs <- err
		}
	}()

	bufferedReader := bufio.NewReader(reader)

	found := false
	for {
		select {
		case err = <-errs:
			return found, errors.Wrap(err, "error reading from journal")
		default:
			line, err := bufferedReader.ReadString('\n')
			if strings.Index(line, text) != -1 {
				found = true
				// Don't return yet; we have to read all the data in the pipe so that
				// the goroutine doesn't block forever.
			}
			if err == io.EOF {
				return found, nil
			} else if err != nil {
				return found, errors.Wrap(err, "error reading from journald pipe")
			}
		}
	}
}

// HasMessage searches the log file and journald for the given message, starting
// at the point of the previous call to HasMessage() (or New() if HasMessage()
// hasn't been called before). text is the plain text message with no newlines;
// regular expressions are not supported, and neither are multi-line messages.
// NOTE: Some race conditions will cause HasMessage to return true twice for a
// single matching message being logged.
func (w *Watcher) HasMessage(ctx context.Context, text string) (bool, error) {
	hasMessageInFile, err := w.hasMessageInFile(text)
	if err != nil {
		return hasMessageInFile, errors.Wrap(err, "error searching text log for message")
	}

	hasMessageInJournald, err := w.hasMessageInJournald(ctx, text)
	if err != nil {
		return hasMessageInFile || hasMessageInJournald, errors.Wrap(err, "error searching journald for message")
	}

	return hasMessageInFile || hasMessageInJournald, nil
}
