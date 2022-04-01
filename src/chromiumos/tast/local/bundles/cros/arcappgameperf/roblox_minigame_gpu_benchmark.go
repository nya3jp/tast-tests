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

	testutil.PerformRobloxTest(ctx, s, username, password, "GPU Benchmark", "roblox-search-benchmark-game-icon.png", func(params testutil.RobloxTestParams) error {
		uda := params.Uda
		// Wait for the "FPS" text which appears in the bottom left when the game is loaded.
		// At this point the screen will be updating frequently so don't wait for stable screenshots.
		if err := uda.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.TextBlock([]string{"FPS"}))(ctx); err != nil {
			return errors.Wrap(err, "failed to load GPU Benchmark")
		}

		perfValues := perf.NewValues()
		if err := testutil.Benchmark(ctx, params.TestParams, gameBenchmarkTime, perfValues); err != nil {
			return errors.Wrap(err, "failed to measure GPU Benchmark")
		}

		// Save the test results.
		fullTestTime := time.Now().Sub(params.ActivityStartTime)
		perfValues.Set(testutil.TestTimePerfMetric(), fullTestTime.Seconds())
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}

		return nil
	})
}
