// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utility functions for writing game performance tests.
package testutil

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/uidetection"
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
}

// RobloxTestParams stores data common to Roblox tests.
type RobloxTestParams struct {
	TestParams
	Uda *uidetection.Context
	Kbd *input.KeyboardEventWriter
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

// BenchmarkResults stores results for the calls to benchmarking.
type BenchmarkResults struct {
	// FPS is a metric that shows average FPS during the sampled period.
	FPS float64 `json:"fps"`
	// CommitDeviation is a metric that shows deviation from the ideal time of committing frames
	// during the sampled period.
	CommitDeviation float64 `json:"commitDeviation"`
	// RenderQuality is a metric in range 0%..100% that shows quality of the render during the
	// sampled period. 100% is ideal quality when frames are produced on time according to FPS.
	RenderQuality float64 `json:"renderQuality"`
}

// PerformTestFunc allows callers to run their desired test after a provided activity has been launched.
type PerformTestFunc func(params TestParams) (err error)

// PerformRobloxTestFunc allows callers to run their desired test after a provided Roblox minigame has be launched.
type PerformRobloxTestFunc func(params RobloxTestParams) (err error)

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
		Arc:               a,
		Device:            d,
		AppPkgName:        appPkgName,
		AppActivityName:   appActivity,
		Activity:          act,
		ActivityStartTime: startTime,
	}); err != nil {
		s.Fatal("Failed to perform test: ", err)
	}
}

// PerformRobloxTest installs Roblox from the play store, starts the activity, searches for a minigame, runs it, and defers to the caller to perform a test.
func PerformRobloxTest(ctx context.Context, s *testing.State, username, password, minigame, icon string, testFunc PerformRobloxTestFunc) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
		// The inputs rendered by Roblox are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 5
	)
	PerformTest(ctx, s, appPkgName, appActivity, func(params TestParams) error {
		// onAppReady: Landing will appear in logcat after the game is fully loaded.
		if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`))); err != nil {
			return errors.Wrap(err, "onAppReady was not found in LogCat")
		}

		kbd, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create keyboard")
		}
		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithTimeout(time.Minute)
		if err := uiauto.Combine("Load Roblox Minigame",
			// Click the button to start the log in process.
			uda.Tap(uidetection.TextBlock([]string{"Log", "In"})),

			// Click the Username/Email/Phone field and type the username.
			uda.Tap(uidetection.Word("Username/Email/Phone")),
			action.Sleep(waitForActiveInputTime),
			kbd.TypeAction(username),

			// Click the password field and type the password.
			uda.Tap(uidetection.Word("Password").First()),
			action.Sleep(waitForActiveInputTime),
			kbd.TypeAction(password),

			// Click the log in button.
			uda.Tap(uidetection.TextBlock(strings.Split("Log In", " ")).First()),

			// A 'verify your account' prompt occasionally shows up. Wait for that and click through if necessary.
			action.IfSuccessThen(
				uda.WithTimeout(time.Second*30).WaitUntilExists(uidetection.TextBlock([]string{"Verify"})),
				uda.Tap(uidetection.TextBlock([]string{"Verify"})),
			),

			// Click the search dialog, type the game name, and hit 'ENTER' to send the query.
			uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-home-screen-search-input.png"))),
			action.Sleep(waitForActiveInputTime),
			kbd.TypeAction(minigame),
			kbd.TypeKeyAction(input.KEY_ENTER),

			// Click the game icon to open the modal.
			uda.Tap(uidetection.CustomIcon(s.DataPath(icon))),

			// Click the 'launch' button in the game modal.
			uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-launch-game.png"))),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to navigate to Roblox minigame")
		}
		return testFunc(RobloxTestParams{TestParams: params, Uda: uda, Kbd: kbd})
	})
}

// displayScaleFactor returns the scale factor for the current display.
func displayScaleFactor(ctx context.Context, tconn *chrome.TestConn) (float64, error) {
	// Find the ratio to convert coordinates in the screenshot to those in the screen.
	screens, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to get the display info")
	}

	scaleFactor, err := screens[0].GetEffectiveDeviceScaleFactor()
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to get the device scale factor")
	}

	return scaleFactor, nil
}

// GetCoords returns an approximate pixel location for the current display and
// given heuristics.
func GetCoords(ctx context.Context, tconn *chrome.TestConn, activityBounds coords.Rect, widthHeuristic, heightHeuristic float64) (coords.Point, error) {
	// Get scale factor, in case the display is scaled.
	scaleFactor, err := displayScaleFactor(ctx, tconn)
	if err != nil {
		return coords.NewPoint(0, 0), errors.Wrap(err, "failed to get scale factor")
	}

	relativeWidth := widthHeuristic / scaleFactor
	relativeHeight := heightHeuristic / scaleFactor
	return coords.NewPoint(int(float64(activityBounds.Width)*relativeWidth), int(float64(activityBounds.Height)*relativeHeight)), nil
}

// StartBenchmarking begins the benchmarking process.
func StartBenchmarking(ctx context.Context, params TestParams) error {
	// Leave the mini-game running for while recording metrics.
	if err := params.TestConn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.arcAppTracingStart)`); err != nil {
		return errors.Wrap(err, "failed to start benchmarking")
	}

	return nil
}

// StopBenchmarking stops the benchmarking process and returns the parsed results.
func StopBenchmarking(ctx context.Context, params TestParams) (results BenchmarkResults, err error) {
	var r BenchmarkResults
	if err := params.TestConn.Call(ctx, &r, `tast.promisify(chrome.autotestPrivate.arcAppTracingStopAndAnalyze)`); err != nil {
		return r, errors.Wrap(err, "failed to stop benchmarking")
	}

	return r, nil
}

// Benchmark starts ARC App tracing, waits for duration, and then stops tracing and updates the provided perf.Values with the BenchmarkResults.
func Benchmark(ctx context.Context, params TestParams, duration time.Duration, perfValues *perf.Values) error {
	if err := StartBenchmarking(ctx, params); err != nil {
		return errors.Wrap(err, "failed to start benchmarking")
	}

	if err := testing.Sleep(ctx, duration); err != nil {
		return errors.Wrap(err, "failed sleep for sample")
	}

	r, err := StopBenchmarking(ctx, params)
	if err != nil {
		return errors.Wrap(err, "failed to stop benchmarking")
	}
	perfValues.Set(FpsPerfMetric(), r.FPS)
	perfValues.Set(CommitDeviationPerfMetric(), r.CommitDeviation)
	perfValues.Set(RenderQualityPerfMetric(), r.RenderQuality*100.0)
	return nil
}

// LaunchTimePerfMetric returns a standard metric that launch time can be saved in.
func LaunchTimePerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "launchTime",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}
}

// LoginTimePerfMetric returns a standard metric that login time can be saved in.
func LoginTimePerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "loginTime",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}
}

// TestTimePerfMetric returns a standard metric that test time can be saved in.
func TestTimePerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "testTime",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}
}

// FpsPerfMetric returns a standard metric that measured FPS can be saved in.
func FpsPerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}
}

// CommitDeviationPerfMetric returns a standard metric that commit deviation can be saved in.
func CommitDeviationPerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "commitDeviation",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}
}

// RenderQualityPerfMetric returns a standard metric that render quality can be saved in.
func RenderQualityPerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "renderQuality",
		Unit:      "percents",
		Direction: perf.BiggerIsBetter,
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

		// Zork.
		"berknip",
		"dirinboz",
		"ezkinil",
		"gumboz",
		"jelboz",
		"jelboz360",
		"morphius",
		"vilboz",
		"vilboz14",
		"vilboz360",
		"woomax",

		// Octopus.
		"ampton",
		"apel",
		"bloog",
		"blooglet",
		"blooguard",
		"blorb",
		"bluebird",
		"bobba",
		"bobba360",
		"casta",
		"dood",
		"dorp",
		"droid",
		"fleex",
		"foob",
		"foob360",
		"garfour",
		"garg",
		"garg360",
		"grabbiter",
		"laser14",
		"lick",
		"meep",
		"mimrock",
		"nospike",
		"orbatrix",
		"phaser",
		"phaser360",
		"sparky",
		"sparky360",
		"vorticon",
		"vortininja",

		// Hatch.
		"nightfury",
		"akemi",
		"dragonair",
		"dratini",
		"helios",
		"jinlon",
		"kindred",
		"kled",
		"kohaku",
		"nightfury",
		"helios",
	}
}
