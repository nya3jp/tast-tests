// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
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
		Func:         AmongUsGpuBenchmark,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Crtaees a local game for Among Us and records performance metrics",
		Contacts:     []string{"pjlee@google.com", "davidwelling@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{},
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
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

func AmongUsGpuBenchmark(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.innersloth.spacemafia"
		appActivity = ""
		// The inputs rendered by Among Us are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 5
		// Stores how long the game should be benchmarked.
		gameBenchmarkTime = time.Minute * 1
	)

	// Data will include settings button picture and "x" button picture.

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) error {
		// Poll for game launched.

		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithTimeout(time.Minute)
		if err := uiauto.Combine("Load GPU Benchmark",
			// Identify and click through optional screens (e.g. download, terms, D.O.B., "play offline" etc.).
			action.IfSuccessThen(
				uda.WithTimeout(time.Second*15).WaitUntilExists(uidetection.TextBlock([]string{"Accept"})),
				uda.Tap(uidetection.TextBlock([]string{"Accept"})),
			),

			action.IfSuccessThen(
				uda.WithTimeout(time.Second*30).WaitUntilExists(uidetection.TextBlock([]string{"I", "Understand"})),
				uda.Tap(uidetection.TextBlock([]string{"I", "Understand"})),
			),

			action.IfSuccessThen(
				uda.WithTimeout(time.Second*15).WaitUntilExists(uidetection.TextBlock([]string{"OK"})),
				uda.Tap(uidetection.TextBlock([]string{"OK"})),
			),

			action.IfSuccessThen(
				uda.WithTimeout(time.Second*15).WaitUntilExists(uidetection.TextBlock([]string{"Play", "Offline"})),
				uda.Tap(uidetection.TextBlock([]string{"Play", "Offline"})),
			),

			// Google Play Games may pop-up after this?

			// Identify and click "x" button to close annoucements pop-up.
			action.IfSuccessThen(
			//uda.WithTimeout(time.Second*30).WaitUntilExists(uidetection.CustomIcon(s.DataPath("x.png"))),
			//uda.Tap(uidetection.CustomIcon(s.DataPath("x.png"))),
			),

			// Identify and click "Local".
			action.Sleep(waitForActiveInputTime),
			uda.Tap(uidetection.TextBlock([]string{"Local"})),

			// Identify and click "Create Game".
			action.Sleep(waitForActiveInputTime),
			uda.Tap(uidetection.TextBlock([]string{"Create", "Game"})),

			// Poll game loaded (wait until settings button appears).
			//uda.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.CustomIcon(s.DataPath("settings.png"))),
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
