// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sender

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
	"chromiumos/tast/testing"
)

const (
	rateDir  = "/var/lib/crash_sender"
	mockPath = "/run/crash_reporter/mock-crash-sending"
)

type MockMode int

const (
	MockSuccess MockMode = iota
	MockFailure
)

// EnableMock tells crash_sender to not send crash reports to the server
// actually. DisableMock must be called after the test is done.
func EnableMock(mode MockMode) error {
	var b []byte
	if mode == MockFailure {
		b = []byte{'1'}
	}
	if err := ioutil.WriteFile(mockPath, b, 0644); err != nil {
		return errors.Wrap(err, "failed to enable crash_sender mock")
	}
	return nil
}

// DisableMock tells crash_sender to send crash reports to the server actually.
func DisableMock() error {
	if err := os.Remove(mockPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to disable crash_sender mock")
	}
	return nil
}

// ResetSentReports clears the number of sent crash reports recorded in
// crash_sender's rate limiting directory.
func ResetSentReports() error {
	if err := os.RemoveAll(rateDir); err != nil {
		return errors.Wrap(err, "failed to reset sent reports")
	}
	return nil
}

// CountSentReports counts the number of sent crash reports recorded in
// crash_sender's rate limiting directory.
func CountSentReports() (int, error) {
	fis, err := ioutil.ReadDir(rateDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, errors.Wrap(err, "failed to count sent reports")
	}
	cnt := 0
	for _, fi := range fis {
		if fi.Mode().IsRegular() {
			cnt++
		}
	}
	return cnt, nil
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

// Run runs crash_sender to process crash dumps stored in crashDir
// and returns the send results by parsing its syslog output.
// crash_sender is run with --ignore_pause_file to ignore the pause file
// created by crash.SetUpCrashTest.
func Run(ctx context.Context, crashDir string) ([]*SendResult, error) {
	sr, err := syslog.NewReader(syslog.Program("crash_sender"))
	if err != nil {
		return nil, err
	}
	defer sr.Close()

	testing.ContextLog(ctx, "Running crash_sender")
	cmd := testexec.CommandContext(ctx,
		"/sbin/crash_sender",
		"--ignore_pause_file",
		"--crash_directory="+crashDir)
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
	payloadKindPattern = regexp.MustCompile(` \((?P<kind>[^)]+)\)$`)
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
					return nil, errors.Errorf("corrupted Metadata line line: %q", e.Content)
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
