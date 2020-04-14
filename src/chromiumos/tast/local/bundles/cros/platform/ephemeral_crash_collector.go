// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type ephemeralCollectionParams struct {
	testFunc     func(context.Context) error
	oobeComplete bool
	consent      bool
}

// System crash directories to test ephemeral collection against.
const (
	rebootVaultCrashDir = "/mnt/stateful_partition/reboot_vault/crash"
	runCrashDir         = "/run/crash_reporter/crash"
	varSpoolCrashDir    = "/var/spool/crash"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: EphemeralCrashCollector,
		Desc: "Verify ephemeral crash collection worked as expected",
		Contacts: []string{
			"sarthakkukreti@google.com",
			"chromeos-storage@google.com",
			"cros-monitoring-forensics@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent", "pstore"},
		Params: []testing.Param{{
			Name: "pre_oobe_collection",
			Val: ephemeralCollectionParams{
				testFunc:     validatePreOobeEphemeralCrashCollection,
				oobeComplete: false,
				consent:      true,
			},
		}, {
			Name: "post_oobe_no_consent",
			Val: ephemeralCollectionParams{
				testFunc:     validatePostOobeEphemeralCrashCollectionWithConsent,
				oobeComplete: true,
				consent:      false,
			},
		}, {
			Name: "post_oobe_with_consent",
			Val: ephemeralCollectionParams{
				testFunc:     validatePostOobeEphemeralCrashCollectionNoConsent,
				oobeComplete: true,
				consent:      true,
			},
		}},
	})
}

// createEphemeralCrashReport creates a fake crash report for the ephemeral crash collector to process.
func createEphemeralCrashReport(ctx context.Context, crashDir, crashName string) error {
	// Create crash directory for ephemeral crash, if it doesn't already exist.
	if _, err := os.Stat(crashDir); os.IsNotExist(err) {
		if err := os.Mkdir(crashDir, 0755); err != nil {
			return errors.Wrapf(err, "failed to create crash directory %s", crashDir)
		}
	}

	crashPath := filepath.Join(crashDir, crashName)

	if err := ioutil.WriteFile(crashPath, []byte(crashName), 0644); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to create fake crash report %s", crashName)
	}

	return nil
}

// runEphemeralCollector runs the ephemeral crash collector.
func runEphemeralCollector(ctx context.Context, preserveAcrossClobber bool) error {
	args := []string{"/sbin/crash_reporter", "--log_to_stderr", "--ephemeral_collect"}
	if preserveAcrossClobber {
		args = append(args, " --preserve_across_clobber")
	}
	cmd := testexec.CommandContext(ctx, "/sbin/crash_reporter", args...)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "crash_reporter ephemeral collect command failed, preserveAcrossClobber : %v", preserveAcrossClobber)
	}
	return nil
}

// expectCrashReport checks the expectation of the existence of the persisted crash report.
func expectCrashReport(ctx context.Context, crashDir, crashName string, exists bool) error {
	crashPath := filepath.Join(crashDir, crashName)

	_, err := os.Stat(crashPath)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to stat() crash file")
	}

	// Existence check should match the parameter passed.
	if exists == os.IsNotExist(err) {
		return errors.Errorf("existence check for crash %s failed: Expected: (%v); Actual (%v)", crashPath, exists, !os.IsNotExist(err))
	}

	// If the file should exist, check the contents.
	if exists {
		defer os.Remove(crashPath)
		contents, err := ioutil.ReadFile(crashPath)
		if err != nil {
			return errors.Wrap(err, "couldn't read crash file")
		}

		if !strings.Contains(string(contents), crashName) {
			return errors.Errorf("crash contents don't match: %s %s", crashName, string(contents))
		}
	}

	return nil
}

// expectEphemeralPreservationAcrossClobber checks if preservation of crashes across clobber works as expected.
func expectEphemeralPreservationAcrossClobber(ctx context.Context, successExpected bool) error {
	if err := createEphemeralCrashReport(ctx, runCrashDir, "fake_crash"); err != nil {
		return errors.Wrap(err, "failed to create crash report")
	}

	if err := runEphemeralCollector(ctx, true); err != nil {
		return err
	}

	// Check the expected contents of the reboot vault match the expected success.
	if err := expectCrashReport(ctx, rebootVaultCrashDir, "fake_crash", successExpected); err != nil {
		return errors.Wrap(err, "crash report check failed")
	}

	return nil
}

// expectEphemeralCollection checks if ephemeral collection from /run works as expected.
func expectEphemeralCollection(ctx context.Context, successExpected bool) error {
	if err := createEphemeralCrashReport(ctx, runCrashDir, "fake_crash_1"); err != nil {
		return errors.Wrap(err, "failed to create crash report")
	}

	if err := createEphemeralCrashReport(ctx, rebootVaultCrashDir, "fake_crash_2"); err != nil {
		return errors.Wrap(err, "failed to create crash report")
	}

	if err := runEphemeralCollector(ctx, false); err != nil {
		return err
	}

	// Check the expected contents of the system crash directory match the expected outcome.
	if err := expectCrashReport(ctx, varSpoolCrashDir, "fake_crash_1", successExpected); err != nil {
		return errors.Wrap(err, "crash report check failed")
	}

	if err := expectCrashReport(ctx, varSpoolCrashDir, "fake_crash_2", successExpected); err != nil {
		return errors.Wrap(err, "crash report check failed")
	}

	return nil
}

// validatePreOobeEphemeralCrashCollection checks ephemeral crash collection before OOBE.
func validatePreOobeEphemeralCrashCollection(ctx context.Context) error {
	// Preservation across clobber is expected to succeed.
	if err := expectEphemeralPreservationAcrossClobber(ctx, true); err != nil {
		return errors.Wrap(err, "failed to validate preservation across clobber")
	}

	// Regular ephemeral report collection is expected to succeed.
	if err := expectEphemeralCollection(ctx, true); err != nil {
		return errors.Wrap(err, "failed to validate ephemeral collection")
	}

	return nil
}

// validatePostOobeEphemeralCrashCollectionWithConsent checks ephemeral crash collection after login (with consent).
func validatePostOobeEphemeralCrashCollectionWithConsent(ctx context.Context) error {
	// Preservation across clobber is expected to succeed.
	if err := expectEphemeralPreservationAcrossClobber(ctx, true); err != nil {
		return errors.Wrap(err, "failed to validate preservation across clobber")
	}

	// Regular ephemeral report collection is expected to succeed.
	if err := expectEphemeralCollection(ctx, true); err != nil {
		return errors.Wrap(err, "failed to validate ephemeral collection")
	}

	return nil
}

// validatePostOobeEphemeralCrashCollectionNoConsent checks ephemeral crash collection after login (without consent).
func validatePostOobeEphemeralCrashCollectionNoConsent(ctx context.Context) error {
	// Preservation across clobber is expected to fail.
	if err := expectEphemeralPreservationAcrossClobber(ctx, false); err != nil {
		return errors.Wrap(err, "failed to validate preservation across clobber")
	}

	// Regular ephemeral report collection is expected to fail.
	if err := expectEphemeralCollection(ctx, false); err != nil {
		return errors.Wrap(err, "failed to validate ephemeral collection")
	}

	return nil
}

func EphemeralCrashCollector(ctx context.Context, s *testing.State) {
	oobeComplete := s.Param().(ephemeralCollectionParams).oobeComplete

	var cr *chrome.Chrome
	var err error

	if oobeComplete {
		cr, err = chrome.New(ctx)
	} else {
		cr, err = chrome.New(ctx, chrome.NoLogin())
	}

	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}

	defer cr.Close(ctx)

	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}

	// Teardown on exiting the test.
	defer crash.TearDownCrashTest(ctx)

	consent := s.Param().(ephemeralCollectionParams).consent

	if !consent {
		// Revoke the consent.
		if err := crash.SetConsent(ctx, cr, false); err != nil {
			s.Fatal("Failed to revoke consent: ", err)
		}
	}

	f := s.Param().(ephemeralCollectionParams).testFunc

	if err := f(ctx); err != nil {
		s.Error("Test failed: ", err)
	}
}
