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

	"chromiumos/tast/errors"
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

	whitelistDir      = "/var/lib/whitelist"
	consentFile       = "/home/chronos/Consent To Send Stats"
	ownerKeyFile      = whitelistDir + "/owner.key"
	signedPolicyFile  = whitelistDir + "/policy"
	pushedPolicyFile  = whitelistDir + "/pushed_policy"
	pushedKeyFile     = whitelistDir + "/pushed_key"
	pushedConsentFile = "/home/chronos/pushed_consent"
)

// setConsent emulates the state where we have consent to send crash reports.
// This creates the file to control whether crash_sender will consider that it
// has consent to send crash reports.
// It also copies a policy blob with the proper policy setting.
func setConsent(ctx context.Context, mockPolicyFilePath string, mockKeyFilePath string) error {
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

// pushConsent pushes the consent file, thus disabling consent.
func pushConsent() error {
	if err := pushFile(signedPolicyFile, pushedPolicyFile); err != nil {
		return err
	}
	if err := pushFile(ownerKeyFile, pushedKeyFile); err != nil {
		return err
	}
	if err := pushFile(consentFile, pushedConsentFile); err != nil {
		return err
	}
	return nil
}

// popConsent pops the consent files, enabling/disabling consent as it was before we pushed the consent.
func popConsent() error {
	if err := popFile(signedPolicyFile, pushedPolicyFile); err != nil {
		return err
	}
	if err := popFile(ownerKeyFile, pushedKeyFile); err != nil {
		return err
	}
	if err := popFile(consentFile, pushedConsentFile); err != nil {
		return err
	}
	return nil
}

func pushFile(origPath string, backupPath string) error {
	if _, err := os.Stat(backupPath); err == nil {
		return errors.Wrapf(err, "backup destination file already exists: %s", backupPath)
	} else if !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to stat backup path")
	}
	if _, err := os.Stat(origPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "failed to stat original file")
	}
	// Create policy file that enables metrics/consent.
	if err := fsutil.MoveFile(origPath, backupPath); err != nil {
		return errors.Wrap(err, "failed to push file")
	}
	return nil
}

func popFile(origPath string, backupPath string) error {
	if f, err := os.Stat(backupPath); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to open backup")
		}
		return os.Remove(origPath)
	} else if !f.Mode().IsRegular() {
		return errors.Wrap(err, "backup is not a regular file")
	}
	if err := fsutil.MoveFile(backupPath, origPath); err != nil {
		return errors.Wrap(err, "failed to pop file")
	}
	return nil
}

// RunCrashTest runs a crash test case after setting up crash reporter.
func RunCrashTest(ctx context.Context, s *testing.State, testFunc func(context.Context, *testing.State)) error {
	if err := pushConsent(); err != nil {
		s.Fatal("Failed to push consent: ", err)
	}
	if err := setConsent(ctx, s.DataPath(MockMetricsOnPolicyFile), s.DataPath(MockMetricsOwnerKeyFile)); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}
	defer func() {
		if err := popConsent(); err != nil {
			s.Fatal("Failed to pop consent: ", err)
		}
	}()
	testFunc(ctx, s)
	return nil
}
