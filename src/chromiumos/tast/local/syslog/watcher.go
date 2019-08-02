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
	"chromiumos/tast/testing"
)

// Watcher allows a test to watch for a specific message in the system log.
// Unlike just running "grep", Watcher will only report messages written after
// the test is started. It also deals with system log rotation.
// TODO(crbug.com/991416) This should also handle messages logged to journal, since
// someday we will move to journald for everything.
type Watcher struct {
	file        *os.File      // The currently open file.
	reader      *bufio.Reader // A Reader wrapping file.
	partialRead string        // If the previous read didn't get to the end-of-line,
	// this is the text we've read so far on that line.
}

// NewWatcher returns a Watcher set to the current point in the file. The next
// call to HasMessage will start looking at the current point in the file.
func NewWatcher(filename string) (*Watcher, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "error opening log file")
	}
	// Note: It's possible this will position us in the middle of a log message
	// (if syslogd is in the middle of writing something out); we don't care
	// because NewWatcher() should be called long before the target message is
	// written to the log file.
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, errors.Wrap(err, "error seeking to end of log file")
	}
	reader := bufio.NewReader(file)
	return &Watcher{file: file, reader: reader}, nil
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
	originalName := w.file.Name()
	stat, err := w.file.Stat()
	if err != nil {
		return false, errors.Wrap(err, "error stat'ing existing file")
	}
	origStat, err := os.Stat(originalName)
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
	file, err := os.Open(originalName)
	if err != nil {
		return false, errors.Wrap(err, "error opening new log file instance")
	}

	w.file.Close()
	w.file = file
	w.reader = bufio.NewReader(file)
	return true, nil
}

// HasMessage searches the log file for the message given to NewWatcher(),
// starting at the point the previous call to HasMessage() found the text (or
// the previous end of the log, if the last HasMessage() didn't find the text;
// or the end of the log at the time NewWatcher() was called, if HasMessage()
// hasn't been called before). text is the plain text message with no newlines;
// regular expressions are not supported, and neither are multi-line messages.
func (w *Watcher) HasMessage(text string) (bool, error) {
	if i := strings.Index(w.partialRead, text); i != -1 {
		// This is possible if a single line contains the target message multiple times.
		w.partialRead = w.partialRead[i+len(text):]
		return true, nil
	}

	if strings.HasSuffix(w.partialRead, "\n") {
		// Only clear partialRead if it ends in a newline. If we got a partial line,
		// we may have gotten the first half of the target text, so we can't clear
		// it, but if partialRead was a complete line, and target text can't have
		// newlines, then we know we can't have the first half of the target text.
		w.partialRead = ""
	}

	for {
		line, err := w.reader.ReadString('\n')
		line = w.partialRead + line
		if i := strings.Index(line, text); i != -1 {
			// Stop reading at the end of the first instance of text; if the text appears
			// twice in the line, return true from the next call to HasMessage as well.
			w.partialRead = line[i+len(text):]
			return true, nil
		}

		if !strings.HasSuffix(line, "\n") {
			// This takes care of the race condition where syslog was halfway through
			// writing the target text when HasMessage was called or when the log was
			// rotated.
			w.partialRead = line
		} else {
			w.partialRead = ""
		}

		if err == io.EOF {
			keepReading, err := w.handleLogRotation()
			if err != nil {
				return false, errors.Wrap(err, "error handling log rotation")
			}
			if !keepReading {
				return false, nil
			}
		} else if err != nil {
			return false, errors.Wrap(err, "error reading log line")
		}
	}
}

// WaitForMessage waits for the watched file to contain text. It returns
// nil when text appears. If the text does not appear by ctx's deadline, a timeout
// error is returned. If text has been added to the file since the last time
// HasMessage or WaitForMessage was called, WaitForMessage returns immediately
// with nil. Like HasMessage, it moves the current read point in the file to just
// after the target message.
func (w *Watcher) WaitForMessage(ctx context.Context, text string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if hasMessage, err := w.HasMessage(text); err != nil {
			return testing.PollBreak(err)
		} else if !hasMessage {
			return errors.Errorf("message %q not found in %s", text, w.file.Name())
		}
		return nil
	}, nil); err != nil {
		return err
	}
	return nil
}
