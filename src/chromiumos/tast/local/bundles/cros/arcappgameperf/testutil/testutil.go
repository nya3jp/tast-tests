// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utility functions for writing game performance tests.
package testutil

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// TestParams stores data common to the tests run in this package.
type TestParams struct {
	TestConn        *chrome.TestConn
	Arc             *arc.ARC
	Device          *ui.Device
	AppPkgName      string
	AppActivityName string
	Activity        *arc.Activity
}

// PollForGameLaunched abstracts away the logic for callers to implement a method
// of checking whether a game has been launched.
type PollForGameLaunched func(params TestParams) (isLaunched bool, err error)

// CleanupOnErrorTime reserves time for cleanup in case of an error.
const CleanupOnErrorTime = time.Second * 30

// PerformLaunchTest installs a game from the play store, times the launching of a game,
// and records the metric for crosbolt. Callers must poll for their own launched state
// by using pollForGameLaunched.
func PerformLaunchTest(ctx context.Context, s *testing.State, appPkgName, appActivity string, pollForGameLaunched PollForGameLaunched) {
	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, CleanupOnErrorTime)
	defer cancel()

	// Pull out the common values.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	// Install the application from the playstore.
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName, 3); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
	}

	// Wait for the CPU to idle before performing the test.
	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
		s.Fatal("Failed to wait until CPU is cooled down: ", err)
	}

	// Take screenshot on failure.
	defer func(ctx context.Context) {
		if s.HasError() {
			captureScreenshot(ctx, s, cr, "failed-launch-test.png")
		}
	}(cleanupCtx)

	// Set up the activity.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}

	defer func(ctx context.Context) {
		act.Close()
	}(cleanupCtx)

	// Start timing and launch the activity.
	startTime := time.Now()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(ctx, tconn)

	// Defer to the caller to determine when the game is launched.
	isLaunched, launchedErr := pollForGameLaunched(TestParams{
		TestConn:        tconn,
		Arc:             a,
		Device:          d,
		AppPkgName:      appPkgName,
		AppActivityName: appActivity,
		Activity:        act,
	})

	// Always take a screenshot of the launched state for debugging purposes.
	captureScreenshot(ctx, s, cr, "launched-state.png")

	if launchedErr != nil {
		s.Fatal("Failed to check launched state: ", launchedErr)
	}

	if isLaunched == false {
		s.Fatal("Activity was not launched")
	}

	// Save the metrics in crosbolt.
	loadTime := time.Now().Sub(startTime)

	perfValues := perf.NewValues()
	perfValues.Set(perf.Metric{
		Name:      "launchTime",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, loadTime.Seconds())

	if perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}

// captureScreenshot takes a screenshot and saves it with the provided filename.
// Since screenshots are useful in debugging but not important to the flow of the test,
// errors are logged rather than bubbled up.
func captureScreenshot(ctx context.Context, s *testing.State, cr *chrome.Chrome, filename string) {
	path := filepath.Join(s.OutDir(), filename)
	if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
		testing.ContextLog(ctx, "Failed to capture screenshot, info: ", err)
	} else {
		testing.ContextLogf(ctx, "Saved screenshot to %s", filename)
	}
}
