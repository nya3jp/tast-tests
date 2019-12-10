// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syslog

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
)

type options struct {
	path string // path to the syslog messages file
}

// Reader allows tests to read syslog messages. It only reports messages written
// after it is started.
// TODO(nya): Deal with system log rotation.
// TODO(crbug.com/991416): This should also handle messages logged to journal,
// since someday we will move to journald for everything.
type Reader struct {
	opts     *options
	file     *os.File      // currently open file
	reader   *bufio.Reader // line-oriented reader wrapping file
	skipNext bool          // if true, skip the next line
	lineBuf  string        // partially read line
}

// Option allows tests to customize the behavior of Reader.
type Option func(*options)

// SourcePath sets the file path to read syslog messages from. The default is
// /var/log/messages.
func SourcePath(p string) Option {
	return func(o *options) {
		o.path = p
	}
}

// Entry represents a log message entry of syslog.
type Entry struct {
	// Timestamp is the time when the log message was emitted.
	Timestamp time.Time
	// Severity indicates the severity of the message, e.g. "ERR".
	Severity string
	// Tag is the TAG part of the message, e.g. "shill[1003]".
	Tag string
	// Program is the program name found in TAG, e.g. "shill".
	Program string
	// PID is the PID found in TAG. It is 0 if missing.
	PID int
	// Content is the CONTENT part of the message.
	Content string
}

// NewReader starts a new Reader that reports syslog messages
// written after it is started. Close must be called after use.
func NewReader(opts ...Option) (r *Reader, retErr error) {
	o := options{
		path: MessageFile,
	}
	for _, opt := range opts {
		opt(&o)
	}

	f, err := os.Open(o.path)
	if err != nil {
		return nil, errors.Wrap(err, "error opening log file")
	}
	defer func() {
		if retErr != nil {
			f.Close()
		}
	}()

	// Seek to 1 byte before the end of the file if the file is not empty.
	//
	// We basically want to seek to the end of the file so that we don't
	// process messages written earlier. Since it is possible that syslogd
	// is in the middle of writing a message, we set skipNext to true.
	// On the other hand, if the last message has been completely written
	// out, then if we seek to the end, the next thing we read will read
	// will be the beginning of the nessage message, the one we want. But
	// since skipNext will be true, we'll discard that message. To avoid
	// skipping a valid next message, we seek to 1 byte before the end of
	// the file to ensure that, in that case, we will read the newline of
	// the last message and clear skipNext.
	fi, err := f.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "failed to tell the file size")
	}
	skipNext := false
	if fi.Size() > 0 {
		if _, err := f.Seek(-1, io.SeekEnd); err != nil {
			return nil, errors.Wrap(err, "failed to seek to end")
		}
		skipNext = true
	}

	return &Reader{
		opts:     &o,
		file:     f,
		reader:   bufio.NewReader(f),
		skipNext: skipNext,
	}, nil
}

// Close closes the Reader.
func (r *Reader) Close() error {
	return r.file.Close()
}

// Read returns the next log message. If the next message is not available yet,
// io.EOF is returned.
func (r *Reader) Read() (*Entry, error) {
	for {
		// ReadString returns err == nil if and only if the returned data
		// ends with a newline.
		line, err := r.reader.ReadString('\n')
		if err == io.EOF {
			// Possible partial read. Keep the data in the buffer.
			r.lineBuf += line
			// TODO(nya): Deal with system log rotation.
			return nil, io.EOF
		} else if err != nil {
			return nil, err
		}

		if r.lineBuf != "" {
			line = r.lineBuf + line
			r.lineBuf = ""
		}

		if r.skipNext {
			r.skipNext = false
			continue
		}

		e, err := parseLine(line)
		if err != nil {
			return nil, err
		}

		return e, nil
	}
}

var (
	linePattern = regexp.MustCompile(`^(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}[+-]\d{2}:\d{2}) (?P<severity>\S+) (?P<tag>.*?): (?P<content>.*)\n$`)
	tagPattern  = regexp.MustCompile(`^(?P<program>[^[]*)\[(?P<pid>\d+)\]$`)
)

// parseLine parses a line in a syslog messages file.
func parseLine(line string) (*Entry, error) {
	ms := linePattern.FindStringSubmatch(line)
	if ms == nil {
		return nil, errors.Errorf("corrupted syslog line: %q", line)
	}
	ts, err := time.Parse(time.RFC3339Nano, ms[1])
	if err != nil {
		return nil, errors.Wrap(err, "corrupted syslog stamp")
	}
	tag := ms[3]
	program := tag
	pid := 0
	if tms := tagPattern.FindStringSubmatch(tag); tms != nil {
		program = tms[1]
		pid, err = strconv.Atoi(tms[2])
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse PID")
		}
	}
	return &Entry{
		Timestamp: ts,
		Severity:  ms[2],
		Tag:       tag,
		Program:   program,
		PID:       pid,
		Content:   ms[4],
	}, nil
}
