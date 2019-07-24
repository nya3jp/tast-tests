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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// setConsent emulates the state where we have consent to send crash reports.
func setConsent(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "/usr/bin/metrics_client", "-C").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to create consent file")
	}
	err := testexec.CommandContext(ctx, "/usr/bin/metrics_client", "-c").Run(testexec.DumpLogOnError)
	if status, ok := testexec.GetWaitStatus(err); !ok {
		return errors.Wrap(err, "failed to get state code from metrics_client")
	} else if status != 0 {
		return errors.Wrap(err, "consent still not enabled")
	}
	return nil
}

// RunCrashTest runs a crash test case after setting up crash reporter.
func RunCrashTest(ctx context.Context, s *testing.State, testFunc func(context.Context, *testing.State)) {
	// Restart session so that no policy prevents sending metrics
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
	if err := setConsent(ctx); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}
	testFunc(ctx, s)
}
