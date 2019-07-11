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
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
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
)

// replaceCrashFilterIn replaces --filter_in= parameter of the crash reporter.
// The kernel is set up to call the crash reporter with the core dump
// as stdin when a process dies. This function adds a filter to the
// command line used to call the crash reporter. This is used to ignore
// crashes in which we have no interest.
//
// This removes any --filter_in= parameter and optionally replaces it
// with a new one.
func replaceCrashFilterIn(param string) error {
	var fileInfo os.FileInfo
	var err error
	if fileInfo, err = os.Stat(CorePattern); err != nil {
		return errors.Wrapf(err, "failed getting core patern file info %s", CorePattern)
	}
	var b []byte
	if b, err = ioutil.ReadFile(CorePattern); err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", CorePattern)
	}
	pattern := strings.TrimSpace(string(b))
	if !strings.HasPrefix(pattern, "|") {
		return errors.Wrapf(err, "pattern should start with '|', but was: %s", pattern)
	}
	re := regexp.MustCompile(` --filter_in=\S*\s*`)
	pattern = re.ReplaceAllString(pattern, "")
	if len(param) != 0 {
		pattern = fmt.Sprintf("%s --filter_in=%s", pattern, strconv.Quote(param))
	}

	if err := ioutil.WriteFile(CorePattern, []byte(pattern), fileInfo.Mode().Perm()); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", CorePattern)
	}
	return nil
}

// DisableCrashFiltering removes the --filter_in argument from the kernel core dump cmdline.
//
// Next time the crash reporter is invoked (due to a crash) it will not
// receive a --filter_in paramter.
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
