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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RobloxMinigameGpuBenchmark,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Logs in to Roblox, loads a mini-game, and records performance metrics",
		Contacts:     []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"roblox-home-screen-search-input.png", "roblox-search-benchmark-game-icon.png", "roblox-launch-game.png"},
		HardwareDeps: hwdep.D(hwdep.Model(testutil.ModelsToTest()...)),
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				Pre:               pre.ArcAppGamePerfBooted,
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Pre:               pre.ArcAppGamePerfBooted,
			}},
		Timeout: 15 * time.Minute,
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password", "arcappgameperf.roblox_username", "arcappgameperf.roblox_password"},
	})
}

func RobloxMinigameGpuBenchmark(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
		// The inputs rendered by Roblox are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 5
		// Stores how long the game should be benchmarked.
		gameBenchmarkTime = time.Minute * 1
	)

	username := s.RequiredVar("arcappgameperf.roblox_username")
	password := s.RequiredVar("arcappgameperf.roblox_password")

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) error {
		// onAppReady: Landing will appear in logcat after the game is fully loaded.
		if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`))); err != nil {
			return errors.Wrap(err, "onAppReady was not found in LogCat")
		}

		kbd, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create keyboard")
		}

		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithTimeout(time.Minute)
		if err := uiauto.Combine("Load GPU Benchmark Minigame",
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
			kbd.TypeAction("GPU Benchmark"),
			kbd.TypeKeyAction(input.KEY_ENTER),

			// Click the game icon to open the modal.
			uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-search-benchmark-game-icon.png"))),

			// Click the 'launch' button in the game modal.
			uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-launch-game.png"))),

			// Wait for the "FPS" text which appears in the bottom left when the game is loaded.
			// At this point the screen will be updating frequently so don't wait for stable screenshots.
			uda.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.TextBlock([]string{"FPS"})),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to finish test")
		}

		// Leave the mini-game running for while recording metrics.
		if err := testutil.StartBenchmarking(ctx, params); err != nil {
			return errors.Wrap(err, "failed to start benchmarking")
		}

		if err := testing.Sleep(ctx, gameBenchmarkTime); err != nil {
			return errors.Wrap(err, "failed sleep for sample")
		}

		r, err := testutil.StopBenchmarking(ctx, params)
		if err != nil {
			return errors.Wrap(err, "failed to stop benchmarking")
		}

		// Save the test results.
		fullTestTime := time.Now().Sub(params.ActivityStartTime)

		perfValues := perf.NewValues()
		perfValues.Set(testutil.TestTimePerfMetric(), fullTestTime.Seconds())
		perfValues.Set(testutil.FpsPerfMetric(), r.FPS)
		perfValues.Set(testutil.CommitDeviationPerfMetric(), r.CommitDeviation)
		perfValues.Set(testutil.RenderQualityPerfMetric(), r.RenderQuality*100.0)
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}

		return nil
	})
}
