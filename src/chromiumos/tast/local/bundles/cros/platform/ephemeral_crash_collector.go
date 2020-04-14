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
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type ephemeralCollectionParams struct {
	oobeComplete bool
	consent      bool
	consentType  crash.ConsentType
}

func init() {
	testing.AddTest(&testing.Test{
		Func: EphemeralCrashCollector,
		Desc: "Verify ephemeral crash collection worked as expected",
		Contacts: []string{
			"sarthakkukreti@google.com",
			"chromeos-storage@google.com",
			"cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pstore"},
		Params: []testing.Param{{
			Name: "pre_oobe_collection",
			Val: ephemeralCollectionParams{
				oobeComplete: false,
				consent:      true,
				consentType:  crash.MockConsent,
			},
		}, {
			Name: "post_oobe_no_consent",
			Val: ephemeralCollectionParams{
				oobeComplete: true,
				consent:      false,
				consentType:  crash.RealConsent,
			},
			ExtraSoftwareDeps: []string{"chrome", "metrics_consent"},
		}, {
			Name: "post_oobe_with_consent",
			Val: ephemeralCollectionParams{
				oobeComplete: true,
				consent:      true,
				consentType:  crash.MockConsent,
			},
		}},
	})
}

// createEphemeralCrashReport creates a fake crash report for the ephemeral crash collector to process.
func createEphemeralCrashReport(ctx context.Context, crashDir, crashName string) error {
	// Create crash directory for ephemeral crash, if it doesn't already exist.
	if _, err := os.Stat(crashDir); os.IsNotExist(err) {
		if err := os.Mkdir(crashDir, 0755); err != nil {
			return errors.Wrapf(err, "failed to create crash directory %q", crashDir)
		}
	}

	crashPath := filepath.Join(crashDir, crashName)

	if err := ioutil.WriteFile(crashPath, []byte(crashName), 0644); err != nil {
		return errors.Wrapf(err, "failed to create fake crash report %q", crashName)
	}

	return nil
}

// runEphemeralCollector runs the ephemeral crash collector.
func runEphemeralCollector(ctx context.Context, preserveAcrossClobber bool) error {
	args := []string{"/sbin/crash_reporter", "--ephemeral_collect", "--log_to_stderr"}
	if preserveAcrossClobber {
		args = append(args, "--preserve_across_clobber")
	}

	if err := testexec.CommandContext(ctx, "/sbin/crash_reporter", args...).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "crash_reporter ephemeral collect command failed")
	}
	return nil
}

// expectCrashReport checks the expectation of the existence of the persisted crash report.
func expectCrashReport(ctx context.Context, crashDir, crashName string, expectExists bool) error {
	crashPath := filepath.Join(crashDir, crashName)
	defer os.Remove(crashPath)

	_, err := os.Stat(crashPath)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to stat() crash file %q", crashPath)
	}

	exists := err == nil

	if exists != expectExists {
		return errors.Errorf("existence check for crash %s failed: Expected: (%v); Actual (%v)", crashPath, exists, !os.IsNotExist(err))
	}

	// If the file should exist, check the contents.
	if exists {
		content, err := ioutil.ReadFile(crashPath)
		if err != nil {
			return errors.Wrapf(err, "couldn't read crash file %q", crashPath)
		}

		if !strings.Contains(string(content), crashName) {
			return errors.Errorf("crash contents don't match: %s %s", crashName, string(content))
		}
	}

	return nil
}

// testEphemeralPreservationAcrossClobber checks if preservation of crashes across clobber succeeded.
func testEphemeralPreservationAcrossClobber(ctx context.Context) error {
	if err := createEphemeralCrashReport(ctx, crash.EarlyCrashDir, "fake_crash"); err != nil {
		return errors.Wrap(err, "failed to create crash report")
	}

	if err := runEphemeralCollector(ctx, true); err != nil {
		return err
	}

	// Check the expected contents of the reboot vault match the expected success.
	if err := expectCrashReport(ctx, crash.ClobberCrashDir, "fake_crash", true); err != nil {
		return errors.Wrap(err, "crash report check failed")
	}

	return nil
}

// testEphemeralCollection checks if ephemeral collection from /run works as expected.
func testEphemeralCollection(ctx context.Context, successExpected bool) error {
	if err := createEphemeralCrashReport(ctx, crash.EarlyCrashDir, "fake_crash_1"); err != nil {
		return errors.Wrap(err, "failed to create crash report")
	}

	if err := createEphemeralCrashReport(ctx, crash.ClobberCrashDir, "fake_crash_2"); err != nil {
		return errors.Wrap(err, "failed to create crash report")
	}

	if err := runEphemeralCollector(ctx, false); err != nil {
		return err
	}

	// Check the expected contents of the system crash directory match the expected outcome.
	if err := expectCrashReport(ctx, crash.SystemCrashDir, "fake_crash_1", successExpected); err != nil {
		return errors.Wrap(err, "crash report check failed")
	}

	if err := expectCrashReport(ctx, crash.SystemCrashDir, "fake_crash_2", successExpected); err != nil {
		return errors.Wrap(err, "crash report check failed")
	}

	return nil
}

func EphemeralCrashCollector(ctx context.Context, s *testing.State) {
	// Restart ui to always be in a logged out state before the test to mimic general usage conditions.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart UI job")
	}

	params := s.Param().(ephemeralCollectionParams)
	var cr *chrome.Chrome

	// Set up chrome for testing ephemeral collection without consent.
	if params.consentType == crash.RealConsent {
		var err error
		var opts []chrome.Option
		if !params.oobeComplete {
			opts = append(opts, chrome.NoLogin())
		}

		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			s.Fatal("Failed to start chrome: ", err)
		}
		defer cr.Close(ctx)
	}

	if !params.consent {
		if err := crash.SetUpCrashTest(ctx); err != nil {
			s.Fatal("SetUpCrashTest failed: ", err)
		}
	} else {
		opt := crash.WithMockConsent()
		if params.consentType == crash.RealConsent && params.consent {
			opt = crash.WithConsent(cr)
		}
		if err := crash.SetUpCrashTest(ctx, opt); err != nil {
			s.Fatal("SetUpCrashTest failed: ", err)
		}
	}
	defer crash.TearDownCrashTest(ctx)

	if !params.consent {
		// Revoke the consent.
		if err := crash.SetConsent(ctx, cr, false); err != nil {
			s.Fatal("Failed to revoke consent: ", err)
		}
	}

	s.Run(ctx, "Preservation across clobber", func(ctx context.Context, s *testing.State) {
		if err := testEphemeralPreservationAcrossClobber(ctx); err != nil {
			s.Fatal("Failed to validate preservation across clobber: ", err)
		}
	})

	s.Run(ctx, "Ephemeral crash collection", func(ctx context.Context, s *testing.State) {
		if err := testEphemeralCollection(ctx, params.consent); err != nil {
			s.Fatal("Failed to validate ephemeral crash collection: ", err)
		}
	})
}
