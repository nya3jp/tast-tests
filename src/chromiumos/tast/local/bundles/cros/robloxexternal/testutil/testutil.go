// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	TestConn          *chrome.TestConn
	ARC               *arc.ARC
	Device            *ui.Device
	AppPkgName        string
	AppActivityName   string
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

// cleanupOnErrorTime reserves time for cleanup in case of an error.
const cleanupOnErrorTime = time.Second * 30

// PerformTest installs a game from the play store, starts the activity, and defers to the caller to perform a test.
func PerformTest(ctx context.Context, s *testing.State, appPkgName, appActivity string, testFunc PerformTestFunc) {
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

	// Install the application from the playstore.
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName, &playstore.Options{}); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
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

	// Set up the activity.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()

	// Start timing and launch the activity.
	startTime := time.Now()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(ctx, tconn)

	// Always take a screenshot of the final state for debugging purposes.
	// This is done with the cleanup context so the main flow is not interrupted.
	defer captureScreenshot(cleanupCtx, s, cr, "final-state.png")

	// Defer to the caller to determine when the game is launched.
	if err := testFunc(TestParams{
		TestConn:          tconn,
		ARC:               a,
		Device:            d,
		AppPkgName:        appPkgName,
		AppActivityName:   appActivity,
		Activity:          act,
		ActivityStartTime: startTime,
	}); err != nil {
		s.Fatal("Failed to perform test: ", err)
	}
}

// LaunchHomeTimePerfMetric returns a standard metric that launch time can be saved in.
func LaunchHomeTimePerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "launchHomeTime",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}
}

// LaunchGameTimePerfMetric returns a standard metric that test time can be saved in.
func LaunchGameTimePerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "launchGameTime",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
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

// ModelsToTest stores the models that are initially relevant for game performance tests.
// TODO(b/206442649): Remove after initial testing is complete.
func ModelsToTest() []string {
	return []string{
		// Eve.
		"eve",

		// Soraka.
		"soraka",

		// Nautilus.
		"nautilus",
		"nautiluslte",

		// Zork.
		"berknip",
		"dirinboz",
		"ezkinil",
		"gumboz",
		"jelboz",
		"jelboz360",
		"morphius",
		"vilboz",
		"vilboz360",
		"woomax",

		// Octopus.
		"ampton",
		"apel",
		"apele",
		"blooglet",
		"blooguard",
		"blorb",
		"bluebird",
		"bobba",
		"bobba360",
		"casta",
		"dorp",
		"droid",
		"laser14",
		"lick",
		"meep",
		"mimrock",
		"nospike",
		"phaser",
		"phaser360",
		"sparky",
		"sparky360",
		"vorticon",
		"vortininja",

		// Hatch.
		"nightfury",
		"akemi",
		"helios",
		"kindred",
		"kohaku",

		// Nami.
		"akali",
		"akali360",
		"pantheon",
		"vayne",

		// Fizz.
		"teemo",
	}
}
