// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crashtest deals with running crash tests.
// Crash tests are tests which crash a user-space program (or the whole
// machine) and generate a core dump. We want to check that the correct crash
// dump is available and can be retrieved.
package crashtest

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	// MockMetricsOnPolicyFile is the name of the mock data file to indicate
	// having consent to send crash reports.
	// A test which calls SetConsent should load this file.
	MockMetricsOnPolicyFile = "crash_tests_mock_metrics_on_policy.bin"

	// MockMetricsOwnerKeyFile is the name of the mock data file used for a
	// policy blob.
	// A test which calls SetConsent should load this file.
	MockMetricsOwnerKeyFile = "crash_tests_mock_metrics_owner.key"
)

// SetConsent emulates the state where we have consent to send crash reports.
// This creates the file to control whether crash_sender will consider that it
// has consent to send crash reports.
// It also copies a policy blob with the proper policy setting.
func SetConsent(ctx context.Context, mockPolicyFilePath string,
	mockKeyFilePath string) error {
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
	// policy file in order to avoid a race condition where chrome
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
