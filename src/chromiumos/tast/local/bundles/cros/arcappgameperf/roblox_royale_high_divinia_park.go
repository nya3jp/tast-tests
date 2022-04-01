// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RobloxRoyaleHighDiviniaPark,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Logs in to Roblox, loads Royale High's Divinia Park and measures how long it can run",
		Contacts:     []string{"davidwelling@google.com", "arc-engprod@google.com"},
		// TODO(b/219524888): Disabled while CAPTCHA prevents test from completing.
		//Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data:         testutil.RobloxMinigameData("roblox-search-royale-high-game-icon.png", "roblox-royale-high-hud.png"),
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
		Timeout: 60 * time.Minute,
		Vars:    []string{"arcappgameperf.RobloxRoyaleHighDiviniaPark.idle_minutes"},
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password", "arcappgameperf.roblox_username", "arcappgameperf.roblox_password"},
	})
}

func RobloxRoyaleHighDiviniaPark(ctx context.Context, s *testing.State) {
	var idleMinutes int = 40
	idleMinutesStr, ok := s.Var("arcappgameperf.RobloxRoyaleHighDiviniaPark.idle_minutes")
	if ok {
		minutes, err := strconv.ParseInt(idleMinutesStr, 10, 32)
		if err != nil {
			s.Fatalf("Failed to parse idle_minutes var %q", idleMinutesStr)
		} else {
			idleMinutes = int(minutes)
		}
	}
	username := s.RequiredVar("arcappgameperf.roblox_username")
	password := s.RequiredVar("arcappgameperf.roblox_password")

	testutil.PerformTest(ctx, s, testutil.RobloxPkgName, testutil.RobloxActivity, func(params testutil.TestParams) error {
		robloxParams, err := testutil.RobloxLogin(ctx, params, username, password)
		if err != nil {
			s.Fatal("Failed to log into Roblox: ", err)
		}
		if err := testutil.RobloxMinigame(ctx, robloxParams, s.DataPath, "Royale High", "roblox-search-royale-high-game-icon.png"); err != nil {
			s.Fatal("Failed to start Royale High minigame: ", err)
		}

		// Navigate from the Royale High login screen to Divinia Park.

		// Royale High has animations running constantly, so we can't wait for
		// stable screenshots. Instead, we WaitUntilExists before every Tap.
		uda := robloxParams.Uda.WithScreenshotStrategy(uidetection.ImmediateScreenshot)
		// The "Explore the World" button that brings up the world map.
		exploreButton := uidetection.TextBlock([]string{"Explore", "the", "World"})
		// The "Divinia Park" button on the world map.
		diviniaParkButton := uidetection.TextBlock([]string{"Divinia", "Park"})
		// The map animates in, so wait a bit for the animation to complete
		// before trying to press any of the buttons on it.
		const royaleHighMapAnimationTime = time.Second * 5
		// The "Visit" button that appears after tapping a location on the world
		// map.
		visitButton := uidetection.TextBlock([]string{"Visit!"})
		// The heads-up-display that shows in-game options while in a Royale
		// High level. When it appears we know Divinia Park has finished
		// loading, and when it disappears we assume the game has stopped.
		hud := uidetection.CustomIcon(s.DataPath("roblox-royale-high-hud.png"))
		if err := uiauto.Combine("Start Divinia Park",
			uda.WaitUntilExists(exploreButton),
			uda.Tap(exploreButton),
			uda.WaitUntilExists(diviniaParkButton),
			action.Sleep(royaleHighMapAnimationTime),
			uda.Tap(diviniaParkButton),
			uda.WaitUntilExists(visitButton),
			uda.Tap(visitButton),
			uda.WithTimeout(time.Minute*5).WaitUntilExists(hud),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to navigate to Divinia Park")
		}

		// Collect FPS and memory metrics for a minute.
		s.Log("Royale High Divinia Park loaded, measure performance")
		basemem, err := metrics.NewBaseMemoryStats(ctx, params.Arc)
		if err != nil {
			s.Fatal("Failed to retrieve base memory stats: ", err)
		}
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
		perfValues := perf.NewValues()
		perfValues.Set(testutil.FpsPerfMetric(), r.FPS)
		perfValues.Set(testutil.CommitDeviationPerfMetric(), r.CommitDeviation)
		perfValues.Set(testutil.RenderQualityPerfMetric(), r.RenderQuality*100.0)
		if err := metrics.LogMemoryStats(ctx, basemem, params.Arc, perfValues, s.OutDir(), "_bench"); err != nil {
			s.Error("Failed to collect memory metrics: ", err)
		}

		// Periodically measure memory and check if Roblox is still running.
		s.Log("Performance test done, start idle memory measurement")
		if err := basemem.Reset(); err != nil {
			s.Fatal("Failed to reset base memory metrics: ", err)
		}
		idleStart := time.Now()
		aliveMinutes := 0
		var lastIdleValues *perf.Values
		for i := 1; i <= idleMinutes; i++ {
			sleep := time.Until(idleStart.Add(time.Minute * time.Duration(i)))
			if sleep <= 0 {
				s.Error("Idle metrics collection took longer than the sleep duration")
				continue
			}
			if err := testing.Sleep(ctx, sleep); err != nil {
				s.Fatal("Failed to sleep during idle: ", err)
			}
			// Log memory metrics separately and then only merge them into
			// perfValues after we know Roblox is still alive.
			idleValues := perf.NewValues()
			if err := metrics.LogMemoryStats(ctx, basemem, params.Arc, idleValues, s.OutDir(), "_idle"); err != nil {
				s.Error("Failed to collect memory metrics: ", err)
			}
			if err := uda.Exists(hud)(ctx); err != nil {
				s.Log("Roblox window disappeared early, HUD not found: ", err)
				break
			}
			aliveMinutes = i
			lastIdleValues = idleValues

			// NB: We do not reset basemem here because each "idle" collection
			// overwrites the previous. We want idle metrics to help diagnose
			// LMKD kills, but also include the whole runtime of the mini-game.
			s.Logf("Idle measurement %d / %d", i, idleMinutes)
			if err := robloxParams.Kbd.TypeKey(ctx, input.KEY_SPACE); err != nil {
				s.Fatal("Failed to press space to not be idle: ", err)
			}
		}
		if lastIdleValues != nil {
			perfValues.Merge(lastIdleValues)
		}
		perfValues.Set(perf.Metric{
			Name:      "time_alive",
			Unit:      "minutes",
			Direction: perf.BiggerIsBetter,
		}, float64(aliveMinutes))

		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}
		return nil
	})
}
