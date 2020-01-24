// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	crashsender "chromiumos/tast/local/crash/sender"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Functions for testing crash_sender. It includes mocking the crash sender, as well as
// verifying the output of the crash sender.

const (
	crashSenderRateDir = "/var/lib/crash_sender"
	crashRunStateDir   = "/run/crash_reporter"
	crashSenderPath    = "/sbin/crash_sender"

	// Used for mocking out the crash sender pretending success or fail for tests.
	// If this file doesn't exist, then the crash sender runs normally. If
	// it exists but is empty, the crash sender will succeed (but actually do
	// nothing). If the file contains something, then the crash sender will fail.
	mockCrashSending = crashRunStateDir + "/mock-crash-sending"
)

var chromeosVersionRegex = regexp.MustCompile("CHROMEOS_RELEASE_VERSION=(.*)")

// SenderOutput represents data extracted from crash sender execution result.
// Wraps crashsender.SendData to add some data fields used by platform.*Crash tests.
type SenderOutput struct {
	SendData     crashsender.SendData
	Output       string // the output from the script
	SendAttempt  bool   // whether the script attempt to send a crash
	SendSuccess  bool   // if it attempted, whether the crash send successful
	Sig          string // signature of the report, or empty string if not given
	SleepTime    int    // if it attempted, how long it slept before sending (if mocked, how long it would have slept). -1 otherwise.
	ReportExists bool   // whether the minidump still exist after calling send script
	RateCount    int    // number of crashes that have been uploaded in the past 24 hours
}

// writeFakeMeta writes a fake meta entry to the system crash directory.
// This is not used unless the call_sender_one_crash is not called with report="".
func writeFakeMeta(name string, execName string, payload string) (string, error) {
	contents := fmt.Sprintf("exec_name=%s\n"+
		"ver=my_ver\n"+
		"payload=%s\n"+
		"done=1\n",
		execName, payload)
	return writeCrashDirEntry(name, []byte(contents))
}

// writeCrashDirEntry Writes a file to the crash directory of the system crash directory with the given name.
// This is used to insert new crash dump files for testing purposes.
func writeCrashDirEntry(name string, contents []byte) (string, error) {
	entry, err := GetCrashDir("root")
	if err != nil {
		return "", errors.Wrapf(err, "failed to get crash dir for user %s", name)
	}
	_, err = os.Stat(SystemCrashDir)
	if err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(SystemCrashDir, os.FileMode(0770)); err != nil {
			return "", errors.Wrapf(err, "failed to create crash directory %s", SystemCrashDir)
		}
	}
	if err := ioutil.WriteFile(entry, contents, 0660); err != nil {
		return "", errors.Wrap(err, "failed to write crash dir entry")
	}
	return entry, nil
}

func waitForProcessEnd(ctx context.Context, name string) error {
	// TODO(crbug.com/1043004): Refine and deduplicate with the same function in
	// /platform/tast-tests/src/chromiumos/tast/local/crash/crash.go
	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "pgrep", "-f", name)
		err := cmd.Run()
		if cmd.ProcessState == nil {
			cmd.DumpLog(ctx)
			return testing.PollBreak(errors.Wrapf(err, "failed to get exit code of %s", name))
		}
		if code := (cmd.ProcessState).ExitCode(); code == 0 {
			// pgrep return code 0: one or more process matched
			return errors.Errorf("still have a %s process", name)
		} else if code != 1 {
			return testing.PollBreak(errors.Errorf("unexpected return code: %d", code))
		}
		// pgrep return code 1: no process matched
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// callSenderOneCrash calls the crash sender script to mock upload one crash.
func callSenderOneCrash(ctx context.Context, cr *chrome.Chrome, username, payloadPath string) (*SenderOutput, error) {
	crashDir, err := GetCrashDir(username)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "Setting SendingMock")
	if err := crashsender.EnableMock(true); err != nil {
		return nil, errors.Wrap(err, "failed to prepare senderOneCrash")
	}
	defer func() {
		if err := crashsender.DisableMock(); err != nil {
			testing.ContextLog(ctx, "Failed at callSenderOneCrash teardown: ", err)
		}
	}()
	report := filepath.Base(payloadPath)
	if report == "" {
		return nil, errors.New("report is empty")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare senderOneCrash")
	}

	if _, err := crashsender.Run(ctx, crashDir); err != nil {
		return nil, errors.Wrap(err, "failed to run crash sender")
	}

	// TODO(crbug.com/970930): Verify sender output.

	reportExists := false
	fileInfo, err := os.Stat(report)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "failed to stat report file %s", report)
	}
	if err == nil {
		if fileInfo.IsDir() {
			return nil, errors.Errorf("report file expected, but %s is a directory", report)
		}
		if err := os.Remove(report); err != nil {
			return nil, errors.Wrap(err, "failed to clean up after mock sending")
		}
		reportExists = true
	}

	return &SenderOutput{
		ReportExists: reportExists,
	}, nil
}

// CheckGeneratedReportSending checks that report sendnig works.
// metaPath and payloadPath, execName, reportKind, and expectedSig specifies the test expectation.
func CheckGeneratedReportSending(ctx context.Context, cr *chrome.Chrome, username, metaPath, payloadPath, execName, reportKind, expectedSig string) error {
	// TODO(crbug.com/970930): Examine content of crashSenderRateDir
	result, err := callSenderOneCrash(ctx, cr, username, payloadPath)
	if err != nil {
		return errors.Wrap(err, "failed to call sender one crash")
	}
	// TODO(crbug.com/970930): Examine more results of crasn sending.
	if result.ReportExists {
		return errors.New("report not sent properly")
	}
	return nil
}

// ResetRateLimiting resets the count of crash reports sent today.
// This clears the contents of the rate limiting directory which has
// the effect of reseting our count of crash reports sent.
func ResetRateLimiting() error {
	if err := os.RemoveAll(crashSenderRateDir); err != nil {
		return errors.Wrapf(err, "failed cleaning crash sender rate dir %s", crashSenderRateDir)
	}
	return nil
}
