// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package robloxexternal

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/robloxexternal/fixtures"
	"chromiumos/tast/local/bundles/cros/robloxexternal/tape"
	"chromiumos/tast/local/bundles/cros/robloxexternal/testutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	appPkgName  = "com.roblox.client"
	appActivity = ".startup.ActivitySplash"
	// The inputs rendered by Roblox are not immediately active after being clicked
	// so wait a moment for the engine to make the input active before interacting with it.
	waitForActiveInputTime = time.Second * 5
	// Stores how long the game should be benchmarked.
	gameBenchmarkTime = time.Minute * 1
	// Stores how long the test should run.
	testTimeout = 15 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ColdMinigameLaunch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Logs in to Roblox, loads a mini-game, and records load times",
		Contacts:     []string{"davidwelling@google.com", "pjlee@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"roblox-search-benchmark-game-icon.png", "roblox-launch-game.png"},
		HardwareDeps: hwdep.D(hwdep.Model(testutil.ModelsToTest()...)), // Only publicly available devices.
		Fixture:      fixtures.ARCAppGamePerfFixture,
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			}},
		Timeout: testTimeout,
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password", tape.ServiceAccountVar},
	})
}

func ColdMinigameLaunch(ctx context.Context, s *testing.State) {
	// Lease an account for the duration of the test.
	account, cleanupLease, err := tape.LeaseGenericAccount(ctx, appPkgName, testTimeout, []byte(s.RequiredVar(tape.ServiceAccountVar)))
	if err != nil {
		s.Fatal("Failed to lease an account: ", err)
	}
	defer cleanupLease(ctx)

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) error {
		// onAppReady: Landing will appear in logcat after the game is fully loaded.
		if err := params.ARC.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`))); err != nil {
			return errors.Wrap(err, "onAppReady was not found in LogCat")
		}

		kbd, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create keyboard")
		}

		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithScreenshotStrategy(uidetection.ImmediateScreenshot).WithTimeout(time.Minute)
		if err := uiauto.Combine("Log into Roblox",
			// Click the button to start the log in process.
			uda.Tap(uidetection.Word("Log").Above(uidetection.Word("Terms"))),

			// Click the Username/Email/Phone field and type the username.
			uda.Tap(uidetection.Word("Username/Email/Phone")),
			action.Sleep(waitForActiveInputTime),
			kbd.TypeAction(account.Username),

			// Click the password field and type the password.
			uda.Tap(uidetection.Word("Password").First()),
			action.Sleep(waitForActiveInputTime),
			kbd.TypeAction(account.Password),

			// Click the log in button.
			uda.Tap(uidetection.TextBlock(strings.Split("Log In", " ")).First()),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to log into Roblox")
		}

		// onAppReady: Home will appear in logcat after the home screen is fully loaded.
		if err := params.ARC.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sHome`))); err != nil {
			return errors.Wrap(err, "onAppReady was not found in LogCat")
		}
		// Record the elapsed time between cold launch and home screen load.
		homeScreenTime := time.Now().Sub(params.ActivityStartTime)

		if err := uiauto.Combine("Load GPU Benchmark Minigame",
			// Click the search dialog, type the game name, and hit 'ENTER' to send the query.
			// To ensure a game with "search" is not accidentally selected, only look
			// above the first "Friends" indicator, which appears above all of the games.
			uda.Tap(uidetection.Word("Search").Above(uidetection.Word("Friends").First())),

			action.Sleep(waitForActiveInputTime),
			kbd.TypeAction("GPU Benchmark"),
			kbd.TypeKeyAction(input.KEY_ENTER),

			// Wait for the search to settle so the right game can be selected.
			// Ideally StableScreenshot could be used but there is a chance that
			// some game icons don't load and instead show a loading animation, which
			// prevents the stable screenshot strategy from being usable.
			action.Sleep(time.Second*15),
			uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-search-benchmark-game-icon.png"))),

			// Click the 'launch' button in the game modal and then wait for it to disappear.
			// Waiting for it to disappear is required because the game dialog shows a screenshot
			// of the game itself, which the detection logic may mistake as the game
			// itself being loaded.
			action.Combine("Start Game",
				uda.WaitUntilExists(uidetection.CustomIcon(s.DataPath("roblox-launch-game.png"))),
				uda.Tap(uidetection.CustomIcon(s.DataPath("roblox-launch-game.png"))),
				uda.WaitUntilGone(uidetection.CustomIcon(s.DataPath("roblox-launch-game.png"))),
			),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to finish test")
		}

		// onGameLoaded: will appear in logcat after the home screen is fully loaded.
		if err := params.ARC.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onGameLoaded:`))); err != nil {
			return errors.Wrap(err, "onGameLoaded was not found in LogCat")
		}
		// Record the elapsed time between cold launch and game launch.
		fullTestTime := time.Now().Sub(params.ActivityStartTime)

		// Save the test results.
		perfValues := perf.NewValues()
		perfValues.Set(testutil.LaunchHomeTimePerfMetric(), homeScreenTime.Seconds())
		perfValues.Set(testutil.LaunchGameTimePerfMetric(), fullTestTime.Seconds())
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}

		return nil
	})
}
