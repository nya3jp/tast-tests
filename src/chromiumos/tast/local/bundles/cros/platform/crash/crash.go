// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains functionality shared by tests that exercise the crash reporter.
package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// CorePattern is the full path of the core pattern file.
	CorePattern = "/proc/sys/kernel/core_pattern"

	// TestCert is a certificate for generating consent to log crash info.
	TestCert = "testcert.p12"

	// CrashReporterPath is the full path of the crash reporter binary.
	CrashReporterPath = "/sbin/crash_reporter"

	// CrashReporterEnabledPath is the full path for crash handling data file.
	CrashReporterEnabledPath = "/var/lib/crash_reporter/crash-handling-enabled"

	crashTestInProgress = "/run/crash_reporter/crash-test-in-progress"
	crasherPath         = "/usr/local/libexec/tast/helpers/local/cros/platform.UserCrash.crasher"

	crashReporterLogFormat = "[user] Received crash notification for %s[%d] sig 11, user %s group %s (%s)"
	crashSenderRateDir     = "/var/lib/crash_sender"
	pauseFile              = "/var/lib/crash_sender_paused"
)

var pidRegex = regexp.MustCompile(`(?m)^pid=(\d+)$`)

// CrasherOptions stores configurations for running crasher process.
type CrasherOptions struct {
	Username   string
	CauseCrash bool
	Consent    bool
}

// CrasherResult stores result status and outputs from a crasher process execution.
type CrasherResult struct {
	// ReturnCode is the return code of the crasher process.
	ReturnCode int

	// Crashed stores whether the crasher returned segv error code.
	Crashed bool

	// CrashReporterCaught stores whether the crash reporter caught a segv.
	CrashReporterCaught bool
}

// DefaultCrasherOptions creates a CrasherOptions which actually cause and catch crash.
// Username is not populated as it should be set explicitly by each test.
func DefaultCrasherOptions() CrasherOptions {
	return CrasherOptions{
		CauseCrash: true,
		Consent:    true,
	}
}

// exitCode extracts exit code from error returned by exec.Command.Run().
// Equivalent to this in Go version >= 1.12: (*cmd.ProcessState).ExitCode()
// This will return code for backward compatibility.
// TODO(yamaguchi): Remove this after golang is uprevved to >= 1.12.
func exitCode(err error) (int, error) {
	e, ok := err.(*exec.ExitError)
	if !ok {
		return 0, errors.Wrap(err, "failed to cast to exec.ExitError")
	}
	s, ok := e.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, errors.Wrap(err, "failed to cast to syscall.WaitStatus")
	}
	if s.Exited() {
		return s.ExitStatus(), nil
	}
	if !s.Signaled() {
		return 0, errors.Wrap(err, "unexpected exit status")
	}
	return -int(s.Signal()), nil
}

// enableSystemSending allows to run system crash_sender.
func enableSystemSending() error {
	if err := os.Remove(pauseFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to remove pause file %s", pauseFile)
	}
	return nil
}

// disableSystemSending disallows to run system crash_sender.
func disableSystemSending() error {
	if f, err := os.Stat(pauseFile); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to stat %s", pauseFile)
		}
		// Create policy file that enables metrics/consent.
		if err := ioutil.WriteFile(pauseFile, []byte{}, 0644); err != nil {
			return errors.Wrap(err, "failed to create pause file")
		}
	} else {
		if !f.Mode().IsRegular() {
			return errors.Errorf("%s was not a regular file", pauseFile)
		}
	}
	return nil
}

// setCrashTestInProgress creates a file to tell crash_repoter that a crash_repoter test is in progress.
func setCrashTestInProgress() error {
	if err := ioutil.WriteFile(crashTestInProgress, []byte("in-progress"), 0644); err != nil {
		return errors.Wrapf(err, "failed writing in-progress state file %s", crashTestInProgress)
	}
	return nil
}

// unsetCrashTestInProgress tells crash_repoter that no crash_repoter test is in progress.
func unsetCrashTestInProgress() error {
	if err := os.Remove(crashTestInProgress); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to remove in-progress state file %s", crashTestInProgress)
	}
	return nil
}

// replaceCrashFilterIn replaces --filter_in= flag value of the crash reporter.
// When param is an empty string, the flag will be removed.
// The kernel is set up to call the crash reporter with the core dump as stdin
// when a process dies. This function adds a filter to the command line used to
// call the crash reporter. This is used to ignore crashes in which we have no
// interest.
func replaceCrashFilterIn(param string) error {
	b, err := ioutil.ReadFile(CorePattern)
	if err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", CorePattern)
	}
	pattern := string(b)
	if !strings.HasPrefix(pattern, "|") {
		return errors.Wrapf(err, "pattern should start with '|', but was: %s", pattern)
	}
	e := strings.Split(strings.TrimSpace(pattern), " ")
	var newargs []string
	replaced := false
	for _, s := range e {
		if !strings.HasPrefix(s, "--filter_in=") {
			newargs = append(newargs, s)
			continue
		}
		if len(param) == 0 {
			// remove from list
			continue
		}
		newargs = append(newargs, "--filter_in="+param)
		replaced = true
	}
	if len(param) != 0 && !replaced {
		newargs = append(newargs, "--filter_in="+param)
	}
	pattern = strings.Join(newargs, " ")

	if err := ioutil.WriteFile(CorePattern, []byte(pattern), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", CorePattern)
	}
	return nil
}

// DisableCrashFiltering removes the --filter_in argument from the kernel core dump cmdline.
// Next time the crash reporter is invoked (due to a crash) it will not receive a
// --filter_in paramter.
func DisableCrashFiltering() error {
	return replaceCrashFilterIn("")
}

// resetRateLimiting resets the count of crash reports sent today.
// This clears the contents of the rate limiting directory which has
// the effect of reseting our count of crash reports sent.
func resetRateLimiting() error {
	if err := os.RemoveAll(crashSenderRateDir); err != nil {
		return errors.Wrapf(err, "failed cleaning crash sender rate dir %s", crashSenderRateDir)
	}
	return nil
}

// setUpTestCrashReporter initializes the crash reporter for test mode.
func setUpTestCrashReporter(ctx context.Context) error {
	// Remove the test status flag to catch real error while initializing and setting up crash reporter.
	if err := unsetCrashTestInProgress(); err != nil {
		return errors.Wrap(err, "failed before initializing crash reporter")
	}
	if err := testexec.CommandContext(ctx, CrashReporterPath, "--init").Run(); err != nil {
		return errors.Wrap(err, "failed to initialize crash reporter")
	}
	// Completely disable crash_reporter from generating crash dumps
	// while any tests are running, otherwise a crashy system can make
	// these tests flaky.
	if err := replaceCrashFilterIn("none"); err != nil {
		return errors.Wrap(err, "failed after initializing crash reporter")
	}
	// Set the test status flag to make crash reporter
	if err := setCrashTestInProgress(); err != nil {
		return errors.Wrap(err, "failed after initializing crash reporter")
	}
	return nil
}

// teardownTestCrashReporter handles resetting some test-specific persistent changes to the system made by setUpTestCrashReporter.
func teardownTestCrashReporter() error {
	if err := DisableCrashFiltering(); err != nil {
		return errors.Wrap(err, "failed while tearing down crash reporter")
	}
	if err := unsetCrashTestInProgress(); err != nil {
		return errors.Wrap(err, "failed while tearing down crash reporter")
	}
	return nil
}

// runCrasherProcess runs the crasher process.
// Will wait up to 10 seconds for crash_reporter to finish.
func runCrasherProcess(ctx context.Context, opts CrasherOptions) (*CrasherResult, error) {
	var command []string
	if opts.Username != "root" {
		command = []string{"su", opts.Username, "-c"}
	}
	basename := filepath.Base(crasherPath)
	if err := replaceCrashFilterIn(basename); err != nil {
		return nil, errors.Wrapf(err, "failed to replace crash filter: %v", err)
	}
	command = append(command, crasherPath)
	if !opts.CauseCrash {
		command = append(command, "--nocrash")
	}
	oldConsent, err := metrics.HasConsent()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get existing consent status: %v", err)
	}
	if oldConsent != opts.Consent {
		metrics.SetConsent(ctx, TestCert, opts.Consent)
		defer metrics.SetConsent(ctx, TestCert, oldConsent)
	}
	cmd := testexec.CommandContext(ctx, command[0], command[1:]...)

	watcher, err := syslog.NewWatcher("/var/log/messages")
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare syslog watcher in runCrasherProcess")
	}

	crasherExitCode := 0
	b, err := cmd.CombinedOutput()
	out := string(b)
	if err != nil {
		var err2 error
		if crasherExitCode, err2 = exitCode(err); err2 != nil {
			return nil, errors.Wrapf(err2, "failed to get crasher exit code: %v", err)
		}
	}

	// Get the PID from the output, since |crasher.pid| may be su's PID.
	m := pidRegex.FindStringSubmatch(out)
	if m == nil {
		return nil, errors.Errorf("no PID found in output: %s", out)
	}
	pid, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse PID from output of command")
	}
	usr, err := user.Lookup(opts.Username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to lookup username %s", opts.Username)
	}
	var reason string
	if opts.Consent {
		reason = "handling"
	} else {
		reason = "ignoring - no consent"
	}
	crashCaughtMessage := fmt.Sprintf(crashReporterLogFormat, basename, pid, usr.Uid, usr.Gid, reason)

	// Wait until no crash_reporter is running.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "pgrep", "-f", "crash_reporter.*:"+basename)
		err := cmd.Run()
		if err == nil {
			return errors.New("still have a process")
		}
		code, err := exitCode(err)
		if err != nil {
			// Failed to extrat exit code.
			cmd.DumpLog(ctx)
			return testing.PollBreak(errors.Wrap(err, "failed to get exit code of crasher"))
		}
		if code == 0 {
			// This will never happen. If return code is 0, cmd.Run indicates it by err==nil.
			return testing.PollBreak(errors.New("inconsistent results returned from cmd.Run()"))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		// TODO(yamaguchi): include log reader message in this error.
		return nil, errors.Wrap(err, "timeout waiting for crash_reporter to finish: ")
	}

	// Wait until crash reporter processes the crash, or making sure it didn't.
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = watcher.WaitForMessage(c, crashCaughtMessage)
	var crashReporterCaught bool
	select {
	case <-c.Done():
		// Context timed out. This usually means WatiForMessage timed out.
		// However, there are two exceptional cases by race conditions.
		// 1. WaitForMessage returned non-timeout error (like I/O error) right before the context times out.
		// Ideally the test should catch it and fail.
		// However we don't distinguish such case with this normal path, because it can only happen after
		// multiple consequent successful reads verifying that the pattern did not appear in the log.
		// 2. WaitForMessage successfully found target message right before context timed out. Covered here.
		crashReporterCaught = err == nil
	default:
		if err != nil {
			return nil, errors.Wrap(err, "failed to verify crash_reporter message")
		}
		crashReporterCaught = true
	}

	var expectedExitCode int
	if opts.Username == "root" {
		// POSIX-style exit code for a signal
		expectedExitCode = -int(syscall.SIGSEGV)
	} else {
		// bash-style exit code for a signal (because it's run with "su -c")
		expectedExitCode = 128 + int(syscall.SIGSEGV)
	}
	result := CrasherResult{
		Crashed:             (crasherExitCode == expectedExitCode),
		CrashReporterCaught: crashReporterCaught,
		ReturnCode:          crasherExitCode,
	}
	testing.ContextLog(ctx, "Crasher process result: ", result)
	return &result, nil
}

// RunCrasherProcessAndAnalyze executes a crasher process and extracts result data from dumps and logs.
func RunCrasherProcessAndAnalyze(ctx context.Context, opts CrasherOptions) (*CrasherResult, error) {
	result, err := runCrasherProcess(ctx, opts)
	if err != nil {
		return result, errors.Wrap(err, "failed to execute and capture result of crasher")
	}
	if !result.Crashed || !result.CrashReporterCaught {
		return result, nil
	}
	// TODO(yamaguchi): Add logic to examine contents of crash dir and store them to result.
	return result, nil
}

// CheckCrashingProcess runs crasher process and verifies that it's processed.
func CheckCrashingProcess(ctx context.Context, opts CrasherOptions) error {
	result, err := RunCrasherProcessAndAnalyze(ctx, opts)
	if err != nil {
		return errors.Wrap(err, "failed to run and analyze crasher")
	}
	if !result.Crashed {
		return errors.Errorf("Crasher returned %d instead of crashing", result.ReturnCode)
	}
	if !result.CrashReporterCaught {
		return errors.New("Logs do not contain crash_reporter message")
	}
	return nil
}

func runCrashTest(ctx context.Context, s *testing.State, testFunc func(context.Context, *testing.State), initialize bool) error {
	if initialize {
		if err := setUpTestCrashReporter(ctx); err != nil {
			return err
		}
		defer teardownTestCrashReporter()
	}
	// Disable crash_sender from running, kill off any running ones.
	// We set a flag to crash_sender when invoking it manually to avoid
	// our invocations being paused.
	if err := disableSystemSending(); err != nil {
		return err
	}
	defer enableSystemSending()
	// Ignore process-not-found error.
	// TODO(yamaguchi): Refactor to this after Go version >= 1.12
	// (*cmd.ProcessState).ExitCode()
	if err := testexec.CommandContext(ctx, "pkill", "-9", "-e", "crash_sender").Run(); err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			return errors.Wrap(err, "failed to get exit status from crash_sender: failed to cast to exec.ExitError")
		}
		s, ok := e.Sys().(syscall.WaitStatus)
		if !ok {
			return errors.Wrap(err, "failed to get exit status from crash_sender: failed to cast to syscall.WaitStatus")
		}
		if s.ExitStatus() != 1 {
			return errors.Wrap(err, "failed to kill crash_sender")
		}
	}
	resetRateLimiting()
	testFunc(ctx, s)
	return nil
}

// RunCrashTests runs crash test cases after setting up crash reporter.
func RunCrashTests(ctx context.Context, s *testing.State, testFuncList []func(context.Context, *testing.State), initialize bool) {
	for _, f := range testFuncList {
		runCrashTest(ctx, s, f, initialize)
	}
}
