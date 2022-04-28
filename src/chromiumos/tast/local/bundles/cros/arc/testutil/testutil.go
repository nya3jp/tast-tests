// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil contains functions and structs used for testing the gaming input overlay.
package testutil

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	apk = "ArcInputOverlayTest.apk"
	pkg = "org.chromium.arc.testapp.inputoverlay"
	cls = "org.chromium.arc.testapp.inputoverlay.MainActivity"

	// cleanupOnErrorTime reserves time for cleanup in case of an error.
	cleanupOnErrorTime = time.Second * 30
)

// TestParams stores data common to the tests run in this package.
type TestParams struct {
	TestConn          *chrome.TestConn
	Arc               *arc.ARC
	Device            *ui.Device
	Activity          *arc.Activity
	ActivityStartTime time.Time
}

// coolDownConfig returns the config to wait for the machine to cooldown for game performance tests.
// This overrides the default config timeout (5 minutes) and temperature threshold (46 C)
// settings to reduce test flakes on low-end devices.
func coolDownConfig() cpu.CoolDownConfig {
	cdConfig := cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)
	cdConfig.PollTimeout = 7 * time.Minute
	cdConfig.TemperatureThreshold = 61000
	return cdConfig
}

// PerformTestFunc allows callers to run their desired test after a provided activity has been launched.
type PerformTestFunc func(params TestParams) (err error)

// SetupTestApp installs the input overlay test application, starts the activity, and defers to the caller to perform a test.
func SetupTestApp(ctx context.Context, s *testing.State, testFunc PerformTestFunc) {
	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupOnErrorTime)
	defer cancel()

	// Pull out the common values.
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	// Install the gaming input overlay test application.
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing ArcInputOverlayTest: ", err)
	}

	// Wait for the CPU to idle before performing the test.
	if _, err := cpu.WaitUntilCoolDown(ctx, coolDownConfig()); err != nil {
		s.Fatal("Failed to wait until CPU is cooled down: ", err)
	}

	// Take screenshot on failure.
	defer func(ctx context.Context) {
		if s.HasError() {
			captureScreenshot(ctx, s, cr, "failed-launch-test.png")
		}
	}(cleanupCtx)

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new ArcInputOverlayTest activity: ", err)
	}
	defer act.Close()

	// Start timing and launch the activity.
	startTime := time.Now()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start ArcInputOverlayTest: ", err)
	}
	defer act.Stop(ctx, tconn)

	// Always take a screenshot of the final state for debugging purposes.
	// This is done with the cleanup context so the main flow is not interrupted.
	defer captureScreenshot(cleanupCtx, s, cr, "final-state.png")

	// Defer to the caller to determine when the game is launched.
	if err := testFunc(TestParams{
		TestConn:          tconn,
		Arc:               a,
		Device:            d,
		Activity:          act,
		ActivityStartTime: startTime,
	}); err != nil {
		s.Fatal("Failed to perform test: ", err)
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
