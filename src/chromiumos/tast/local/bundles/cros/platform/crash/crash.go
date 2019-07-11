// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash deals with running crash tests.
// Crash tests are tests which crash a user-space program (or the whole
// machine) and generate a core dump. We want to check that the correct crash
// dump is available and can be retrieved.
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
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// CorePattern is the full path of the core pattern file.
	CorePattern = "/proc/sys/kernel/core_pattern"

	// CrashReporterPath is the full path of the crash reporter binary.
	CrashReporterPath = "/sbin/crash_reporter"

	// MockMetricsOnPolicyFile is the name of the mock data file to indicate
	// having consent to send crash reports.
	// A test which calls SetConsent should load this file.
	MockMetricsOnPolicyFile = "crash_tests_mock_metrics_on_policy.bin"

	// MockMetricsOwnerKeyFile is the name of the mock data file used for a
	// policy blob.
	// A test which calls SetConsent should load this file.
	MockMetricsOwnerKeyFile = "crash_tests_mock_metrics_owner.key"

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

// SetConsent emulates the state where we have consent to send crash reports.
// This creates the file to control whether crash_sender will consider that it
// has consent to send crash reports.
// It also copies a policy blob with the proper policy setting.
func SetConsent(ctx context.Context, mockPolicyFilePath string, mockKeyFilePath string) error {
	const (
		whitelistDir     = "/var/lib/whitelist"
		consentFile      = "/home/chronos/Consent To Send Stats"
		ownerKeyFile     = whitelistDir + "/owner.key"
		signedPolicyFile = whitelistDir + "/policy"
	)
	if e, err := os.Stat(whitelistDir); err == nil && e.IsDir() {
		// Create policy file that enables metrics/consent.
		if err := fsutil.CopyFile(mockPolicyFilePath, signedPolicyFile); err != nil {
			return err
		}
		if err := fsutil.CopyFile(mockKeyFilePath, ownerKeyFile); err != nil {
			return err
		}
	}
	// Create deprecated consent file.  This is created *after* the
	// policy file in order to avoid a race condition where Chrome
	// might remove the consent file if the policy's not set yet.
	// We create it as a temp file first in order to make the creation
	// of the consent file, owned by chronos, atomic.
	// See crosbug.com/18413.
	tempFile := consentFile + ".tmp"
	if err := ioutil.WriteFile(tempFile, []byte("test-consent"), 0644); err != nil {
		return err
	}

	if err := os.Chown(tempFile, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return err
	}
	if err := os.Rename(tempFile, consentFile); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Created ", consentFile)
	return nil
}

// clearSpooledCrashes clears system and user crash directories.
// This removes all crash reports which are waiting to be sent.
func clearSpooledCrashes() error {
	matches, err := filepath.Glob(userCrashDirs)
	if err != nil {
		return errors.Wrapf(err, "failed globing user crash dirs %s", userCrashDirs)
	}
	for _, dir := range append(matches, systemCrashDir, fallbackUserCrashDir) {
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed cleaning a crash dir %s", dir)
		}
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

// RunCrashTest runs a crash test case after setting up crash reporter.
func RunCrashTest(ctx context.Context, s *testing.State, testFunc func(context.Context, *testing.State)) error {
	if err := initializeCrashReporter(ctx); err != nil {
		return err
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
			return errors.Wrap(err, "failed to get exit status from crash_sender")
		}
		s, ok := e.Sys().(syscall.WaitStatus)
		if !ok {
			return errors.Wrap(err, "failed to get exit status from crash_sender")
		}
		if s.ExitStatus() != 1 {
			return errors.Wrap(err, "failed to kill crash_sender")
		}
	}
	testFunc(ctx, s)
	if err := clearSpooledCrashes(); err != nil {
		return errors.Wrap(err, "failed to clear spooled crashes")
	}
	return nil
}
