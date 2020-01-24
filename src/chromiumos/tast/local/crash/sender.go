// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/syslog"
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

// senderOptions contains options for callSenderOneCrash.
type senderOptions struct {
	SendSuccess bool   // Mock a successful send if true
	Report      string // report to use for crash, if --None-- we create one.
	ShouldFail  bool   // expect the crash_sender program to fail
	IgnorePause bool   // crash_sender should ignore pause file existence
}

// DefaultSenderOptions creates a senderOptions object with default values.
func DefaultSenderOptions() senderOptions {
	return senderOptions{
		SendSuccess: true,
		ShouldFail:  false,
		IgnorePause: true,
	}
}

// enableSendingMock enables mocking of the sending process.
// See the description of mockCrashSending at the top of this file.
// sendSuccess decides whether the mock sends success or failure.
func enableSendingMock(sendSuccess bool) error {
	data := "" // Empty content, indicates success
	if !sendSuccess {
		data = "1" // Non-empty, indicates failure
	}
	if err := ioutil.WriteFile(mockCrashSending, []byte(data), 0644); err != nil {
		return errors.Wrap(err, "failed to create pause file")
	}
	return nil
}

// disableSendingMock disables mocking of the sending process.
func disableSendingMock() error {
	if err := os.Remove(mockCrashSending); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to remove mock crash file %s", mockCrashSending)
	}
	return nil
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

// waitForProcessEnd waits until all processes that match pattern by process name ends.
func waitForProcessEnd(ctx context.Context, name string) error {
	// TODO(crbug.com/1043004): Deduplicate with the similar function in
	// src/chromiumos/tast/local/bundles/cros/platform/crash/crash.go
	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "pgrep", name)
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

// waitForSenderCompletion waits for no crash_sender's last message to be placed in the
// system log before continuing and for the process to finish.
// Otherwise we might get only part of the output.
func waitForSenderCompletion(ctx context.Context, reader *syslog.Reader) error {
	if _, err := reader.Wait(ctx, 60*time.Second, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, "crash_sender done.")
	}); err != nil {
		return errors.Wrap(err, "crash_sender completion log did not appear")
	}
	c, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := waitForProcessEnd(c, "crash_sender"); err != nil {
		return errors.Wrap(err, "crash_sender process did not end correctly")
	}
	return nil
}

// getDmpContents creates the contents of the dmp file for our made crashes.
// The dmp file contents are deliberately large and hard-to-compress. This
// ensures logging_CrashSender hits its bytes/day cap before its sends/day
// cap.
func getDmpContents() []byte {
	// Matches kDefaultMaxUploadBytes
	const maxCrashSize = 1024 * 1024
	result := make([]byte, maxCrashSize, maxCrashSize)
	rand.Read(result)
	return result
}

// prepareSenderOneCrash creates a fake crash report for testing purposes.
// report is the report to use for crash. If it's empty string, we create one.
func prepareSenderOneCrash(ctx context.Context, cr *chrome.Chrome, report string) (string, error) {
	// Use the same file format as crash does normally:
	// <basename>.#.#.#.meta
	const fakeTestBasename = "fake.1.2.3"
	if report == "" {
		payload, err := writeCrashDirEntry(fmt.Sprintf("%s.dmp", fakeTestBasename), getDmpContents())
		if err != nil {
			return "", errors.Wrap(err, "failed while preparing sender one crash")
		}
		report, err = writeFakeMeta(fmt.Sprintf("%s.meta", fakeTestBasename), "fake", payload)
		if err != nil {
			return "", errors.Wrap(err, "failed while preparing sender one crash")
		}
	}
	return report, nil
}

// callSenderOneCrash calls the crash sender script to mock upload one crash.
func callSenderOneCrash(ctx context.Context, cr *chrome.Chrome, opts senderOptions) error {
	testing.ContextLog(ctx, "Setting SendingMock")
	if err := enableSendingMock(opts.SendSuccess); err != nil {
		return errors.Wrap(err, "failed to prepare senderOneCrash")
	}
	defer func() {
		if err := disableSendingMock(); err != nil {
			testing.ContextLog(ctx, "Failed at callSenderOneCrash teardown: ", err)
		}
	}()
	report, err := prepareSenderOneCrash(ctx, cr, opts.Report)
	opts.Report = report
	if err != nil {
		return errors.Wrap(err, "failed to prepare senderOneCrash")
	}
	w, err := syslog.NewReader(syslog.Program("crash_sender"))
	if err != nil {
		return errors.Wrap(err, "failed to create syslog reader")
	}

	var option string
	if opts.IgnorePause {
		option = "--ignore_pause_file"
	}
	cmd := testexec.CommandContext(ctx, crashSenderPath, option)
	scriptOutput, err := cmd.CombinedOutput()
	if code, ok := testexec.ExitCode(err); !ok {
		return errors.Wrap(err, "failed to get exit code of crash_sender")
	} else if code != 0 && !opts.ShouldFail {
		return errors.Errorf("crash_sender returned an unexpected non-zero value (%d)", code)
	}

	if err := waitForSenderCompletion(ctx, w); err != nil {
		return err
	}
	if string(scriptOutput) != "" {
		testing.ContextLogf(ctx, "crash_sender stdout/stderr: %q", scriptOutput)
	}

	// TODO(crbug.com/970930): Parse sender output and return it.
	return nil
}

// CheckGeneratedReportSending checks that report sendnig works.
// metaPath and payloadPath, execName, reportKind, and expectedSig specifies the test expectation.
func CheckGeneratedReportSending(ctx context.Context, cr *chrome.Chrome, metaPath, payloadPath, execName, reportKind, expectedSig string) error {
	o := DefaultSenderOptions()
	o.Report = filepath.Base(payloadPath)
	if err := callSenderOneCrash(ctx, cr, o); err != nil {
		return errors.Wrap(err, "failed to call sender one crash")
	}
	// TODO(crbug.com/970930): Verify the result of callSenderOneCrash.
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
