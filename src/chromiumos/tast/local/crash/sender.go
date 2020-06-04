// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	// SendRecordDir is the path to the directory containing send record files.
	// A send record file represents an upload event of a crash report. Its content is
	// serialized crash.SendRecord protocol buffers message, and its timestamp indicates
	// when the upload was performed.
	SendRecordDir = "/var/lib/crash_sender"

	mockSendingPath = "/run/crash_reporter/mock-crash-sending"
)

// EnableMockSending tells crash_sender to not send crash reports to the server
// actually. If success is true, crash_sender always emulates successful uploads;
// otherwise it emulates failed uploads.
func EnableMockSending(success bool) error {
	return enableMockSending(mockSendingPath, success)
}

func enableMockSending(path string, success bool) error {
	var b []byte
	if !success {
		b = []byte{'1'}
	}
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		return errors.Wrap(err, "failed to enable crash_sender mock")
	}
	return nil
}

// disableMockSending tells crash_sender to send crash reports to the server actually.
// Usually path should be mockSendingPath.
func disableMockSending(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to disable crash_sender mock")
	}
	return nil
}

// resetSendRecords clears send record files under path. Usually path should be SendRecordDir.
func resetSendRecords(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return errors.Wrap(err, "failed to reset sent reports")
	}
	return nil
}

// ListSendRecords returns a list of send record files under SendRecordDir.
// A send record file represents an upload event of a crash report. Its content is
// serialized crash.SendRecord protocol buffers message, and its timestamp indicates
// when the upload was performed.
func ListSendRecords() ([]os.FileInfo, error) {
	fis, err := ioutil.ReadDir(SendRecordDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to count send records")
	}
	var rs []os.FileInfo
	for _, fi := range fis {
		if fi.Mode().IsRegular() {
			rs = append(rs, fi)
		}
	}
	return rs, nil
}

// SendResult is the result of crash_sender sending a single crash dump entry.
type SendResult struct {
	Schedule time.Time
	Success  bool
	Data     SendData
}

// SendData is the data of a single crash dump entry sent to the server by
// crash_sender.
type SendData struct {
	MetadataPath string
	PayloadPath  string
	PayloadKind  string
	Product      string
	Version      string
	Board        string
	HWClass      string
	Executable   string
	ImageType    string
	BootMode     string
}

// RunSender runs crash_sender to process pending crash dumps and returns the send
// results by parsing its syslog output.
// crash_sender is run with --ignore_pause_file to ignore the pause file
// created by crash.SetUpCrashTest.
func RunSender(ctx context.Context) ([]*SendResult, error) {
	return runSenderWithArgs(ctx, "--ignore_pause_file")
}

// RunSenderNoIgnorePauseFile is similar to RunSender but does not instruct crash_sender to
// ignore the pause file.
func RunSenderNoIgnorePauseFile(ctx context.Context) ([]*SendResult, error) {
	return runSenderWithArgs(ctx)
}

func runSenderWithArgs(ctx context.Context, args ...string) ([]*SendResult, error) {
	sr, err := syslog.NewReader(ctx, syslog.Program("crash_sender"))
	if err != nil {
		return nil, err
	}
	defer sr.Close()

	testing.ContextLog(ctx, "Running: crash_sender ", shutil.EscapeSlice(args))
	cmd := testexec.CommandContext(ctx, "/sbin/crash_sender", args...)
	// crash_sender does not output anything to stdout/stderr. For debugging,
	// always proceed to syslog processing.
	runErr := cmd.Run()

	var es []*syslog.Entry
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for {
			e, err := sr.Read()
			if err == io.EOF {
				return errors.New("crash_sender end message not seen yet")
			}
			if err != nil {
				return testing.PollBreak(err)
			}
			// Log crash_sender syslog entries for debugging.
			testing.ContextLog(ctx, "crash_sender: ", e.Content)
			es = append(es, e)
			// crash_sender runs its main function in minijail, so this message is
			// printed by two processes. Catch the message from the parent process.
			// Otherwise, when running crash_sender multiple times, tests can
			// confuse two messages from one crash_sender run with those from
			// two crash_sender runs.
			if e.PID == cmd.Process.Pid && strings.Contains(e.Content, "crash_sender done.") {
				return nil
			}
		}
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for crash_sender reports")
	}

	if runErr != nil {
		return nil, errors.Wrap(runErr, "crash_sender failed (see logs for output)")
	}

	return parseLogs(es)
}

func parseLogs(es []*syslog.Entry) ([]*SendResult, error) {
	// startReportLog appears at the beginning of each send result. We split the
	// syslog entries using this string.
	const startReportLog = "Evaluating crash report:"
	var rs []*SendResult
	for i := 0; i < len(es); i++ {
		if !strings.HasPrefix(es[i].Content, startReportLog) {
			continue
		}
		j := i + 1
		for ; j < len(es); j++ {
			if strings.HasPrefix(es[j].Content, startReportLog) {
				break
			}
		}
		r, err := parseLogsForResult(es[i:j])
		if err != nil {
			return nil, err
		}
		rs = append(rs, r)
		i = j - 1
	}
	return rs, nil
}

var (
	schedulePattern    = regexp.MustCompile(`Scheduled to send in (\d+)s`)
	payloadKindPattern = regexp.MustCompile(` \(([^)]+)\)$`)
)

func parseLogsForResult(es []*syslog.Entry) (*SendResult, error) {
	var r SendResult
	for _, e := range es {
		if m := schedulePattern.FindStringSubmatch(e.Content); m != nil {
			sec, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, errors.Wrapf(err, "corrupted report line: %q", e.Content)
			}
			r.Schedule = e.Timestamp.Add(time.Duration(sec) * time.Second)
		} else if strings.Contains(e.Content, "Mocking successful send") {
			r.Success = true
		} else if strings.HasPrefix(e.Content, "  ") {
			// This is a key-value pair.
			kv := strings.SplitN(strings.TrimSpace(e.Content), ": ", 2)
			if len(kv) != 2 {
				return nil, errors.Errorf("corrupted report line: %q", e.Content)
			}
			k, v := kv[0], kv[1]
			switch k {
			case "Metadata":
				m := payloadKindPattern.FindStringSubmatch(v)
				if m == nil {
					return nil, errors.Errorf("corrupted metadata line: %q", e.Content)
				}
				r.Data.MetadataPath = strings.TrimSuffix(v, m[0])
				r.Data.PayloadKind = m[1]
			case "Payload":
				r.Data.PayloadPath = v
			case "Product":
				r.Data.Product = v
			case "Version":
				r.Data.Version = v
			case "Board":
				r.Data.Board = v
			case "HWClass":
				r.Data.HWClass = v
			case "Exec name":
				r.Data.Executable = v
			case "Image type":
				r.Data.ImageType = v
			case "Boot mode":
				r.Data.BootMode = v
			}
		}
	}
	return &r, nil
}
