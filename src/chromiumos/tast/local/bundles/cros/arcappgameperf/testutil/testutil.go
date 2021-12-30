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
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// TestParams stores data common to the tests run in this package.
type TestParams struct {
	TestConn          *chrome.TestConn
	Arc               *arc.ARC
	Device            *ui.Device
	AppPkgName        string
	AppActivityName   string
	Activity          *arc.Activity
	ActivityStartTime time.Time
	Context           context.Context
}

// LoginHeuristics stores data about the location of various fields.
type LoginHeuristics struct {
	FirstLoginButton  float64
	UsernameField     float64
	UsernameString    string
	PasswordField     float64
	PasswordString    string
	SecondLoginButton float64
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
	defer act.Close()

	// Start timing and launch the activity.
	startTime := time.Now()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
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
		AppPkgName:        appPkgName,
		AppActivityName:   appActivity,
		Activity:          act,
		ActivityStartTime: startTime,
		Context:           ctx,
	}); err != nil {
		s.Fatal("Failed to perform test: ", err)
	}
}

// PerformLogin assumes a fully launched activity, performs a login, and defers to the caller to perform a test.
func PerformLogin(ctx context.Context, s *testing.State, params TestParams, loginHeuristics LoginHeuristics, testFunc PerformTestFunc) {
	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupOnErrorTime)
	defer cancel()

	// Pull out the common values.
	cr := s.PreValue().(arc.PreData).Chrome

	// Wait for the CPU to idle before performing the test.
	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
		s.Fatal("Failed to wait until CPU is cooled down: ", err)
	}

	// Take screenshot on failure.
	defer func(ctx context.Context) {
		if s.HasError() {
			captureScreenshot(ctx, s, cr, "failed-login-test.png")
		}
	}(cleanupCtx)

	// Start up keyboard.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()

	// Derive activity and test connection.
	act := params.Activity
	tconn := params.TestConn

	//captureScreenshot(cleanupCtx, s, cr, "initial-state.png")

	// Click screen anywhere.
	if err = mouse.Click(tconn, getCoords(ctx, act, s, loginHeuristics.FirstLoginButton), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click first login button: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Locate/click on login button.
	//if err = mouse.Click(tconn, getCoords(ctx, act, loginHeuristics.FirstLoginButton), mouse.LeftButton)(ctx); err != nil {
	if err = mouse.Click(tconn, getCoords(ctx, act, s, loginHeuristics.FirstLoginButton), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click first login button: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	//captureScreenshot(cleanupCtx, s, cr, "login1.png")

	// Locate/click username field, and type username.
	if err = mouse.Click(tconn, getCoords(ctx, act, s, loginHeuristics.UsernameField), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click username field: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err = mouse.Click(tconn, getCoords(ctx, act, s, loginHeuristics.UsernameField), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click username field: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err = ew.Type(ctx, loginHeuristics.UsernameString); err != nil {
		s.Fatal("Failed to write username: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	//captureScreenshot(cleanupCtx, s, cr, "username.png")

	// Locate/click password field, and enter.
	if err = mouse.Click(tconn, getCoords(ctx, act, s, loginHeuristics.PasswordField), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click password field: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err = ew.Type(ctx, loginHeuristics.PasswordString); err != nil {
		s.Fatal("Failed to write password: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	//captureScreenshot(cleanupCtx, s, cr, "password.png")

	// Locate login button, start timer, and click login button.
	if err = mouse.Click(tconn, getCoords(ctx, act, s, loginHeuristics.SecondLoginButton), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click second login button: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err = mouse.Click(tconn, getCoords(ctx, act, s, loginHeuristics.SecondLoginButton), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click second login button: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	//captureScreenshot(cleanupCtx, s, cr, "inputs-attempted.png")

	// Reassign start time.
	startTime := time.Now()

	// Defer to the caller to determine when the game is logged in.
	if err := testFunc(TestParams{
		TestConn:          tconn,
		Arc:               params.Arc,
		Device:            params.Device,
		AppPkgName:        params.AppPkgName,
		AppActivityName:   params.AppActivityName,
		Activity:          act,
		ActivityStartTime: startTime,
	}); err != nil {
		s.Fatal("Failed to perform test (post login): ", err)
	}
}

// LaunchTimePerfMetric returns a standard metric that launch time can be saved in.
func LaunchTimePerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "launchTime",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}
}

// getCoords returns an approximate pixel location given an activity and a
// heuristic.
func getCoords(ctx context.Context, act *arc.Activity, s *testing.State, heuristic float64) coords.Point {
	bounds, err := act.SurfaceBounds(ctx)
	if err == nil {
		s.Fatal("Failed to get surface bounds: ", err)
	}
	if bounds.Width > 0 || bounds.Height > 0 {
		s.Fatal("Bounds should be positive: ", err)
	}

	ScreenWidth := bounds.Width
	ScreenHeight := bounds.Height

	width := int(float64(ScreenWidth*0+2256) * 0.35)
	height := int(float64(ScreenHeight*0+1504) * heuristic)

	return coords.Point{width, height}
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
