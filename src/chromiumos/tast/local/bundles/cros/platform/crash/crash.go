// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains functionality shared by tests that exercise the crash reporter.
package crash

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
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

	crashSenderRateDir = "/var/lib/crash_sender"
	pauseFile          = "/var/lib/crash_sender_paused"

	fallbackUserCrashDir = "/home/chronos/crash"
	pauseFile            = "/var/lib/crash_sender_paused"
	systemCrashDir       = "/var/spool/crash"
	userCrashDirs        = "/home/chronos/u-*/crash"
)

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

// replaceCrashFilterIn replaces --filter_in= flag value of the crash reporter.
// When param is an empty string, the flag will be removed.
// The kernel is set up to call the crash reporter with the core dump as stdin
// when a process dies. This function adds a filter to the command line used to
// call the crash reporter. This is used to ignore crashes in which we have no
// interest. The order of the commandline arguments will not be preserved.
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
		newargs = append(newargs, "--filter_in="+strconv.Quote(param))
		replaced = true
	}
	if len(param) != 0 && !replaced {
		newargs = append(newargs, "--filter_in="+strconv.Quote(param))
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

// clearSpooledCrashes clears system and user crash directories.
// This removes all crash reports which are waiting to be sent.
func clearSpooledCrashes() error {
	if err := os.RemoveAll(systemCrashDir); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed cleaning system crash dir %s", systemCrashDir)
	}
	matches, err := filepath.Glob(userCrashDirs)
	if err != nil {
		return errors.Wrapf(err, "failed globing user crash dirs %s", userCrashDirs)
	}
	for _, match := range matches {
		if err := os.RemoveAll(match); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed cleaning a user crash dir %s", match)
		}
	}
	if err := os.RemoveAll(fallbackUserCrashDir); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed cleaning fallback user crash dir %s", fallbackUserCrashDir)
	}
	return nil
}

// initializeCrashReporter starts up the crash reporter.
func initializeCrashReporter(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, CrashReporterPath, "--init").Run(); err != nil {
		return errors.Wrap(err, "failed to initialize crash reporter")
	}
	// Completely disable crash_reporter from generating crash dumps
	// while any tests are running, otherwise a crashy system can make
	// these tests flaky.
	return replaceCrashFilterIn("none")
}

func runCrashTest(ctx context.Context, s *testing.State, testFunc func(context.Context, *testing.State), initialize bool) error {
	if initialize {
		if err := initializeCrashReporter(ctx); err != nil {
			return err
		}
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
	return clearSpooledCrashes()
}

// TearDown resets some system state after test case run.
func TearDown(ctx context.Context) error {
	return enableSystemSending()
}
