// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AmongusGpuBenchmark,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Creates a local game for Among Us and records performance metrics",
		Contacts:     []string{"pjlee@google.com", "davidwelling@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"amongus-dob-ok-button.png", "amongus-in-game-settings-button.png", "amongus-announcements-close-button.png"},
		HardwareDeps: hwdep.D(hwdep.Model(testutil.ModelsToTest()...)),
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				Pre:               pre.ArcAppGamePerfBooted,
			}, {
				// The hardware dependency for the "vm" subtest and the entire "vm_zork" subtest
				// are put in place to prevent this test from being run on 4GB zork devices with
				// zork-arc-r, which have been shown to have memory issues with this test (b/224785022).

				// TODO(b/224785022): Remove this hardware dependency as well as the "vm_zork" subtest once
				// a fix for 4GB zork-arc-r devices for this test has been found and implemented, or the
				// "MinMemoryForPlatforms" hardware dependency is merged.
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("zork")),
				Pre:               pre.ArcAppGamePerfBooted,
			}, {
				Name:              "vm_zork",
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("zork"), hwdep.MinMemory(5000)),
				Pre:               pre.ArcAppGamePerfBooted,
			}},
		Timeout: 20 * time.Minute,
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

func AmongusGpuBenchmark(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.innersloth.spacemafia"
		appActivity = ".EosUnityPlayerActivity"
		// The inputs rendered by Among Us are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 5
		// Stores how long the game should be benchmarked.
		gameBenchmarkTime = time.Minute * 1
		// The amount of time that the test should wait for the optional play games
		// prompts which need to be closed.
		playGamesClosePromptTimeout = time.Second * 45
	)

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) error {
		// No need to poll for game launched.
		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)

		if err := uiauto.Combine("Tap through optional screens",
			// Identify and click through optional screens (e.g. download, terms, D.O.B., "play offline").

			// Click through optional screen for downloading data.
			action.IfSuccessThen(
				uda.WaitUntilExists(uidetection.TextBlock([]string{"Accept"})),
				uda.WithScreenshotStrategy(uidetection.ImmediateScreenshot).Tap(uidetection.TextBlock([]string{"Accept"})),
			),

			// Click through optional screen for EULA, allowing extended time for slow downloads.
			action.IfSuccessThen(
				uda.WithTimeout(time.Minute*5).WaitUntilExists(uidetection.TextBlock([]string{"Understand"})),
				uda.Tap(uidetection.TextBlock([]string{"Understand"})),
			),

			// Click through date of birth screen.
			// TODO(b/220912392): Icon detection coordinates inconsistent with that of word/text detection.
			action.IfSuccessThen(
				uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("amongus-dob-ok-button.png"))),
				uda.Tap(uidetection.Word("OK").Nth(1)),
			),

			// Click through game mode, with the "offline" option.
			action.IfSuccessThen(
				uda.WaitUntilExists(uidetection.TextBlock([]string{"Offline"})),
				uda.Tap(uidetection.TextBlock([]string{"Offline"})),
			),

			// Google Play Games may pop-up after this. It could contain a 'Cancel' or 'Not now' prompt.
			action.IfSuccessThen(
				uda.WithTimeout(playGamesClosePromptTimeout).WaitUntilExists(uidetection.Word("CANCEL")),
				uda.Tap(uidetection.Word("CANCEL")),
			),

			action.IfSuccessThen(
				testutil.WaitForExists(params.Device.Object(ui.Text("Not now"), ui.ClassName("android.widget.Button")), playGamesClosePromptTimeout),
				testutil.Click(params.Device.Object(ui.Text("Not now"), ui.ClassName("android.widget.Button"))),
			),

			// Identify and click "x" button to close announcements pop-up.
			action.IfSuccessThen(
				uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("amongus-announcements-close-button.png"))),
				uda.Tap(uidetection.CustomIcon(s.DataPath("amongus-announcements-close-button.png"))),
			),

			// Poll created menu loaded (wait until "LOCAL" appears).
			uda.WaitUntilExists(uidetection.Word("LOCAL")),
		)(ctx); err != nil {
			return errors.Wrap(err, "menu not loaded")
		}

		if err := uiauto.Combine("Click Local",
			// Identify and click "Local".
			uda.Tap(uidetection.Word("LOCAL")),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click past initial menu")
		}

		if err := uiauto.Combine("Load GPU Benchmark",
			// Identify and click "Create Game".
			action.Sleep(waitForActiveInputTime),
			uda.Tap(uidetection.TextBlock([]string{"Create", "Game"})),

			// Poll created game loaded (wait until settings button appears).
			// A failure to join a created session will be retried twice before being skipped.
			action.IfSuccessThen(
				action.Not(uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("amongus-in-game-settings-button.png")))),
				uiauto.Retry(2, uiauto.Combine("Close announcements and click 'Create game' again",
					uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("amongus-announcements-close-button.png"))),
					uda.Tap(uidetection.CustomIcon(s.DataPath("amongus-announcements-close-button.png"))),
					uda.Tap(uidetection.TextBlock([]string{"Create", "Game"})),
					uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("amongus-in-game-settings-button.png"))),
				)),
			),
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
