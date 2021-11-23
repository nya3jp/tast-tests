// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syslog

import (
	"bufio"
	"context"
	"io"
	"os"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// LineReader is a common helper for Reader and ChromeReader. It handles getting
// each time and handles issues like log rotation and partially-written lines.
type LineReader struct {
	path     string        // path to the syslog messages file
	file     *os.File      // currently open file
	reader   *bufio.Reader // line-oriented reader wrapping file
	skipNext bool          // if true, skip the next line
	lineBuf  string        // partially read line
}

// NewLineReader starts a new LineReader that reports log messages.
// If fromStart is false only reports log messages written after
// the reader is created. Close must be called after use.
// opts is used to customise polling for the file to exist e.g. in case the log
// is being rotated at the time we try to open it. Pass nil to get defaults.
func NewLineReader(ctx context.Context, path string, fromStart bool, opts *testing.PollOptions) (r *LineReader, retErr error) {
	var f *os.File
	// Avoid race conditions around log rotation. If the file doesn't exist at
	// the moment we try to open it, retry until it does.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		f, err = os.Open(path)
		return err
	}, opts)
	if err != nil {
		return nil, errors.Wrap(err, "error opening log file")
	}
	defer func() {
		if retErr != nil {
			f.Close()
		}
	}()

	skipNext := false
	if !fromStart {
		// Seek to 1 byte before the end of the file if the file is not empty.
		//
		// We basically want to seek to the end of the file so that we don't
		// process messages written earlier. Since it is possible that syslogd (or
		// Chrome) is in the middle of writing a message, we set skipNext to true.
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
		if fi.Size() > 0 {
			if _, err := f.Seek(-1, io.SeekEnd); err != nil {
				return nil, errors.Wrap(err, "failed to seek to end")
			}
			skipNext = true
		}
	}

	return &LineReader{
		path:     path,
		file:     f,
		reader:   bufio.NewReader(f),
		skipNext: skipNext,
	}, nil
}

// Close closes the LineReader.
func (r *LineReader) Close() error {
	return r.file.Close()
}

// ReadLine returns the next log line. If the next message is not available yet,
// io.EOF is returned.
func (r *LineReader) ReadLine() (string, error) {
	for {
		// ReadString returns err == nil if and only if the returned data
		// ends with a newline.
		line, err := r.reader.ReadString('\n')
		if err == io.EOF {
			// Possible partial read. Keep the data in the buffer.
			r.lineBuf += line
			keepReading, err := r.handleLogRotation()
			if err != nil {
				return "", errors.Wrap(err, "error handling log rotation")
			}
			if !keepReading {
				return "", io.EOF
			}
			// Log was rotated, continue reading the next file.
			continue
		} else if err != nil {
			return "", err
		}

		if r.lineBuf != "" {
			line = r.lineBuf + line
			r.lineBuf = ""
		}

		if r.skipNext {
			r.skipNext = false
			continue
		}

		return line, nil
	}
}

// handleLogRotation should be called when reading hits EOF. It checks to see if
// the current log has been rotated (that is, if /var/log/messages has been
// moved to /var/log/messages.1 and a new /var/log/messages created). If it has,
// the Reader is pointed at the new instance, and the caller is told to keep
// reading.
func (r *LineReader) handleLogRotation() (keepReading bool, err error) {
	stat, err := r.file.Stat()
	if err != nil {
		return false, errors.Wrap(err, "error stat'ing existing file")
	}
	origStat, err := os.Stat(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Old log file was moved, but new file has not yet been created. Next
			// call to Read() will come back in here and try again to open the
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
	// /var/log/messages.2 in between two ReadLine() calls).
	file, err := os.Open(r.path)
	if err != nil {
		return false, errors.Wrap(err, "error opening new log file instance")
	}

	r.file.Close()
	r.file = file
	r.reader = bufio.NewReader(file)
	return true, nil
}

// EntryPred is a predicate of Entry. It should return false if
// e should be dropped.
type EntryPred func(e *Entry) bool

// options contains options for creating a Reader (but not a ChromeReader)
type options struct {
	path    string      // path to the syslog messages file
	filters []EntryPred // predicates to filter syslog entries
}

// Reader allows tests to read syslog messages. It only reports messages written
// after it is started. It also deals with system log rotation.
type Reader struct {
	lineReader *LineReader
	filters    []EntryPred // predicates to filter syslog entries
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

// ProgramName encloses names of the program in custom type.
type ProgramName string

const (
	// Chrome filter.
	Chrome ProgramName = "chrome"

	// CrashReporter filter.
	CrashReporter ProgramName = "crash_reporter"

	// Cryptohomed filter.
	Cryptohomed ProgramName = "cryptohomed"

	// Cupsd filter.
	Cupsd ProgramName = "cupsd"

	// CrashSender filter.
	CrashSender ProgramName = "crash_sender"
)

// Program instructs Reader to report messages from a certain program only.
func Program(name ProgramName) Option {
	return func(o *options) {
		o.filters = append(o.filters, func(e *Entry) bool {
			return e.Program == string(name)
		})
	}
}

// SeverityName encloses names of the severity in custom type.
type SeverityName string

const (
	// Verbose1 filter.
	Verbose1 SeverityName = "VERBOSE1"

	// Verbose2 filter.
	// We should never be enabling VERBOSE3+ in system logs.
	Verbose2 SeverityName = "VERBOSE2"

	// Info filter.
	Info SeverityName = "INFO"

	// Debug filter.
	Debug SeverityName = "DEBUG"

	// Notice filter.
	Notice SeverityName = "NOTICE"

	// Warning filter.
	Warning SeverityName = "WARNING"

	// Err filter.
	Err SeverityName = "ERR"
)

// Severities instructs Reader to report messages from certain severities only.
func Severities(names ...SeverityName) Option {
	return func(o *options) {
		o.filters = append(o.filters, func(e *Entry) bool {
			for _, name := range names {
				if e.Severity == string(name) {
					return true
				}
			}
			return false
		})
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

	// Line is the raw syslog line. It always ends with a newline character.
	Line string
}

// ParseError is returned when a log line failed to parse.
type ParseError struct {
	*errors.E

	// Line is a raw log line which failed to parse. It always ends with a
	// newline character.
	Line string
}

// NewReader starts a new Reader that reports syslog messages
// written after it is started. Close must be called after use.
func NewReader(ctx context.Context, opts ...Option) (r *Reader, retErr error) {
	o := options{
		path: MessageFile,
	}
	for _, opt := range opts {
		opt(&o)
	}

	lineReader, err := NewLineReader(ctx, o.path, false, nil)
	if err != nil {
		return nil, err
	}

	return &Reader{
		lineReader: lineReader,
		filters:    o.filters,
	}, nil
}

// Close closes the Reader.
func (r *Reader) Close() error {
	return r.lineReader.Close()
}

// Read returns the next log message.
// If the next message is not available yet, io.EOF is returned. If the next
// line is read successfully but it failed to parse, *ParseError is returned.
func (r *Reader) Read() (*Entry, error) {
	for {
		line, err := r.lineReader.ReadLine()
		if err != nil {
			return nil, err
		}

		e, err := parseSyslogLine(line)
		if err != nil {
			return nil, &ParseError{
				E:    errors.Wrap(err, "failed to parse syslog line"),
				Line: line,
			}
		}

		ok := true
		for _, f := range r.filters {
			if !f(e) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}

		return e, nil
	}
}

// ErrNotFound is returned when no matching message is found by a Wait call.
var ErrNotFound = errors.New("no matching message found")

// Wait waits until it finds a log message matching f.
// If Wait returns successfully, the next call of Read or Wait will continue
// processing messages from the message immediately following the matched
// message. Otherwise the position of the Reader is somewhere between the
// starting position and the end of the file.
func (r *Reader) Wait(ctx context.Context, timeout time.Duration, f EntryPred) (*Entry, error) {
	var entry *Entry
	err := testing.Poll(ctx, func(ctx context.Context) error {
		for {
			e, err := r.Read()
			if err == io.EOF {
				return ErrNotFound
			}
			if err != nil {
				return testing.PollBreak(err)
			}
			if f(e) {
				entry = e
				return nil
			}
		}
	}, &testing.PollOptions{Timeout: timeout})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
	}
	return entry, err
}

var (
	// TODO(crbug.com/1144594): Remove backward compatibility once enough time
	// has passed after switching to UTC timestamp.
	// linePattern will match log with UTC timestamp (Z) and local timestamp for
	// backward compatibility.
	linePattern = regexp.MustCompile(`^(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}(?:Z|[+-]\d{2}:\d{2})) (?P<severity>\S+) (?P<tag>.*?): (?P<content>.*)\n$`)
	tagPattern  = regexp.MustCompile(`^(?P<program>[^[]*)\[(?P<pid>\d+)\]$`)
)

// parseSyslogLine parses a line in a syslog messages file.
func parseSyslogLine(line string) (*Entry, error) {
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
		Line:      line,
	}, nil
}

// ChromeReader allows tests to read Chrome-format syslog messages (such as
// /var/log/chrome/chrome). It only reports messages written after it is started.
// It also deals with Chrome restart causing /var/log/chrome/chrome to point to
// a new file.
type ChromeReader struct {
	lineReader *LineReader
}

// ChromeEntry represents a log message entry of a Chrome-format messages.
type ChromeEntry struct {
	// Severity indicates the severity of the message, e.g. "ERROR".
	Severity string
	// PID is the process ID of the Chrome process that wrote the message.
	PID int
	// Content is the CONTENT part of the message. For multi-line messages, only
	// the first line is included.
	Content string
}

// NewChromeReader starts a new ChromeReader that reports Chrome log messages
// written after it is started. Close must be called after use.
func NewChromeReader(ctx context.Context, path string) (r *ChromeReader, retErr error) {
	lineReader, err := NewLineReader(ctx, path, false, nil)
	if err != nil {
		return nil, err
	}

	return &ChromeReader{
		lineReader: lineReader,
	}, nil
}

// Close closes the ChromeReader.
func (r *ChromeReader) Close() error {
	return r.lineReader.Close()
}

// Read returns the next log message. If the next message is not available yet,
// io.EOF is returned.
func (r *ChromeReader) Read() (*ChromeEntry, error) {
	for {
		line, err := r.lineReader.ReadLine()
		if err != nil {
			return nil, err
		}

		e, ok := parseChromeLine(line)
		if !ok {
			continue
		}

		return e, nil
	}
}

var (
	chromeLinePattern       = regexp.MustCompile(`^\[(?P<pid>\d+):\d+:\d{4}/\d{6}.\d{6}:(?P<severity>[^:]+):[^\]]+\] (?P<content>.*)\n$`)
	chromeSyslogLinePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z (?P<severity>\S+) \S+?\[(?P<pid>\d+):\d+\]: \[\S+\] (?P<content>.*)\n$`)
)

// parseChromeLine parses a line in a Chrome-style messages file. Unlike
// parseSyslogLine, parseChromeLine never returns an error, but just a nil
// ChromeEntry. Since Chrome log entries may have newlines in them, we expect that
// some lines will be unparseable, and don't stop the file reading if we find
// such lines.
// Chrome will switch from a linux style timestamp and format to a
// syslog compatible fomrat. This looks for a syslog match first, then tries
// the older Chrome format.
func parseChromeLine(line string) (*ChromeEntry, bool) {
	ms := chromeSyslogLinePattern.FindStringSubmatch(line)
	if ms != nil {
		pid, err := strconv.Atoi(ms[2])
		if err != nil {
			return nil, false
		}

		return &ChromeEntry{
			Severity: ms[1],
			PID:      pid,
			Content:  ms[3],
		}, true
	}

	ms = chromeLinePattern.FindStringSubmatch(line)
	if ms == nil {
		return nil, false
	}
	pid, err := strconv.Atoi(ms[1])
	if err != nil {
		return nil, false
	}
	return &ChromeEntry{
		Severity: ms[2],
		PID:      pid,
		Content:  ms[3],
	}, true
}
