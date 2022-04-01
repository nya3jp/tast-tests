// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
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
		// TODO(b/219524888): Disabled while CAPTCHA prevents test from completing.
		//Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data:         testutil.RobloxMinigameData("roblox-search-benchmark-game-icon.png"),
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
	username := s.RequiredVar("arcappgameperf.roblox_username")
	password := s.RequiredVar("arcappgameperf.roblox_password")

	testutil.PerformTest(ctx, s, testutil.RobloxPkgName, testutil.RobloxActivity, func(params testutil.TestParams) error {
		robloxParams, err := testutil.RobloxLogin(ctx, params, username, password)
		if err != nil {
			s.Fatal("Failed to log into Roblox: ", err)
		}
		if err := testutil.RobloxMinigame(ctx, robloxParams, s.DataPath, "GPU Benchmark", "roblox-search-benchmark-game-icon.png"); err != nil {
			s.Fatal("Failed to start GPU Benchmark: ", err)
		}
		uda := robloxParams.Uda

		// Wait for the "FPS" text which appears in the bottom left when the game is loaded.
		// At this point the screen will be updating frequently so don't wait for stable screenshots.
		if err := uda.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.TextBlock([]string{"FPS"}))(ctx); err != nil {
			return errors.Wrap(err, "failed to load GPU Benchmark")
		}

		// Leave the mini-game running for while recording metrics.
		if err := testutil.StartBenchmarking(ctx, params); err != nil {
			return errors.Wrap(err, "failed to start benchmarking")
		}

		const gameBenchmarkTime = time.Minute * 1
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
