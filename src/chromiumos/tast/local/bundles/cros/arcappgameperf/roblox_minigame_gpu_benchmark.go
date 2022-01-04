// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RobloxMinigameGpuBenchmark,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Captures launch metrics for Roblox",
		Contacts:     []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"roblox-home-screen-search-input.png", "roblox-search-benchmark-game-icon.png", "roblox-launch-game.png"},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				Pre:               pre.ArcAppGamePerfBooted,
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Pre:               pre.ArcAppGamePerfBooted,
			}},
		Timeout: 10 * time.Minute,
		// TODO: Replace with arcappgameperf variables.
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

// TODO: Extract out to some shared method so this data can be used by other tests.
type runtimeStats struct {
	// FPS is a metric that shows average FPS during the sampled period.
	FPS float64 `json:"fps"`
	// CommitDeviation is a metric that shows deviation from the ideal time of commiting frames
	// during the sampled period.
	CommitDeviation float64 `json:"commitDeviation"`
	// RenderQuality is a metric in range 0%..100% that shows quality of the rander during the
	// sampled period. 100% is ideal quality when frames are produced on time according to FPS.
	RenderQuality float64 `json:"renderQuality"`
}

func RobloxMinigameGpuBenchmark(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
	)

	// TODO: Needs registered values in private repo.
	username := ""
	password := ""

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) error {
		// onAppReady: Landing will appear in logcat after the game is fully loaded.
		if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`))); err != nil {
			return errors.Wrap(err, "onAppReady was not found in LogCat")
		}

		kbd, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create keyboard")
		}

		uda := uidetection.NewDefault(params.TestConn)
		uda.ScreenshotStrategy(uidetection.NormalScreenshot)

		if err := uiauto.Combine("Load GPU Benchmark Minigame",
			// Click the button to start the log in process.
			uda.WaitUntilExists(uidetection.TextBlock(strings.Split("Log In", " "))),
			uda.Tap(uidetection.TextBlock([]string{"Log", "In"})),

			// Click the Username/Email/Phone field.
			uda.WaitUntilExists(uidetection.Word("Username/Email/Phone")),
			uda.Tap(uidetection.Word("Username/Email/Phone")),

			// Type the username after the input is ready to accept keyboard input.
			action.Sleep(time.Second*2),
			kbd.TypeAction(username),

			// Click the password field.
			uda.Tap(uidetection.Word("Password").First()),

			// Type the password after the input is ready to accept keyboard input.
			action.Sleep(time.Second*2),
			kbd.TypeAction(password),

			// Click the log in button.
			uda.Tap(uidetection.TextBlock(strings.Split("Log In", " "))),

			// A 'verify your account' prompt occasionally shows up. Wait for that and click through if necessary.
			action.IfSuccessThen(
				uda.WaitUntilExists(uidetection.TextBlock([]string{"Verify"})),
				uda.Tap(uidetection.TextBlock([]string{"Verify"})),
			),

			// Click the search dialog.
			uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("roblox-home-screen-search-input.png"))),
			uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-home-screen-search-input.png"))),

			// Search for the game. Pause before typing to give the input type to accept input.
			action.Sleep(time.Second*2),
			kbd.TypeAction("GPU Benchmark"),
			kbd.TypeKeyAction(input.KEY_ENTER),

			// Click the game icon to open the modal.
			uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("roblox-search-benchmark-game-icon.png"))),
			uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-search-benchmark-game-icon.png"))),

			// Click the 'launch' button in the game modal.
			uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("roblox-launch-game.png"))),
			uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-launch-game.png"))),

			// Wait for the "FPS" text which appears in the bottom left when the game is loaded.
			action.Sleep(time.Second*2),
			uda.WaitUntilExists(uidetection.TextBlock([]string{"FPS"})),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to finish test")
		}

		// Leave the mini-game running for 15 seconds while recording metrics.
		if err := params.TestConn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.arcAppTracingStart)`); err != nil {
			return errors.Wrap(err, "failed to start trace")
		}

		if err := testing.Sleep(ctx, time.Second*15); err != nil {
			return errors.Wrap(err, "failed sleep for sample")
		}

		var r runtimeStats
		if err := params.TestConn.Call(ctx, &r, `tast.promisify(chrome.autotestPrivate.arcAppTracingStopAndAnalyze)`); err != nil {
			return errors.Wrap(err, "failed to stop trace")
		}

		// Output the results of the test.
		fullTestTime := time.Now().Sub(params.ActivityStartTime)

		// TODO: Standardize metric names.
		perfValues := perf.NewValues()
		perfValues.Set(perf.Metric{
			Name:      "testTime",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, fullTestTime.Seconds())
		perfValues.Set(perf.Metric{
			Name:      "fps",
			Unit:      "fps",
			Direction: perf.BiggerIsBetter,
		}, r.FPS)
		perfValues.Set(perf.Metric{
			Name:      "commitDeviation",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, r.CommitDeviation)
		perfValues.Set(perf.Metric{
			Name:      "renderQuality",
			Unit:      "percents",
			Direction: perf.BiggerIsBetter,
		}, r.RenderQuality*100.0)

		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}

		return nil
	})
}
