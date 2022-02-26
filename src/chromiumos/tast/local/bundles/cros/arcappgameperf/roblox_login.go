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
		Func:         RobloxLogin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Captures login metrics for Roblox",
		Contacts:     []string{"davidwelling@google.com", "pjlee@google.com", "arc-engprod@google.com"},
		// TODO(b/219524888): Disabled while CAPTCHA prevents test from completing.
		//Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		// TODO(b/206442649): Remove after initial testing is complete.
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
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password", "arcappgameperf.roblox_username", "arcappgameperf.roblox_password"},
	})
}

func RobloxLogin(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
		// The inputs rendered by Roblox are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 5
		// Stores how long the game should be benchmarked.
		gameBenchmarkTime = time.Minute * 1
	)

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) error {
		// Get Username and Password for Roblox.
		username := s.RequiredVar("arcappgameperf.roblox_username")
		password := s.RequiredVar("arcappgameperf.roblox_password")

		// Start up keyboard and ACUITI.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to open keyboard")
		}
		defer kb.Close()

		uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithTimeout(time.Minute)

		// Make sure Roblox is launched.
		// onAppReady: Landing will appear in logcat after the game is fully loaded.
		if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`))); err != nil {
			return errors.Wrap(err, "onAppReady was not found in LogCat")
		}

		if err := uiauto.Combine("Enter login information",
			// Click login button.
			uda.Tap(uidetection.TextBlock([]string{"Log", "In"})),

			// Click username field and enter username.
			uda.Tap(uidetection.Word("Username/Email/Phone")),
			action.Sleep(waitForActiveInputTime),
			kb.TypeAction(username),

			// Click password field and enter password.
			uda.Tap(uidetection.Word("Password").First()),
			action.Sleep(waitForActiveInputTime),
			kb.TypeAction(password),

			// Click the log in button.
			uda.Tap(uidetection.TextBlock(strings.Split("Log In", " ")).First()),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to finish test")
		}

		// Start timer for metrics.
		startTime := time.Now()

		// onAppReady: AvatarExperienceLandingPage will appear in logcat after the game is fully logged in.
		if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sAvatarExperienceLandingPage`))); err != nil {
			return errors.Wrap(err, "\"onAppReady: AvatarExperienceLandingPage\" was not found in LogCat")
		}

		// Save the metric in crosbolt.
		loginTime := time.Now().Sub(startTime)
		perfValues := perf.NewValues()
		perfValues.Set(testutil.LoginTimePerfMetric(), loginTime.Seconds())

		if err := perfValues.Save(s.OutDir()); err != nil {
			return errors.Wrap(err, "failed to save performance values")
		}

		return nil
	})
}
