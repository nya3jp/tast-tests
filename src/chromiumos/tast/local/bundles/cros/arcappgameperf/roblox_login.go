// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RobloxLogin,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Captures launch metrics for Roblox",
		Contacts:     []string{"davidwelling@google.com", "pjlee@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
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
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

func RobloxLogin(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
	)

	// Hard coded login field heuristics.
	const (
		FirstLoginButton  = 0.0
		UsernameField     = 0.0
		UsernameString    = ""
		PasswordField     = 0.0
		PasswordString    = ""
		SecondLoginButton = 0.0
	)

	const loginHeuristics = testutil.LoginHeuristics{FirstLoginButton, UsernameField, UsernameString, PasswordField, PasswordString, SecondLoginButton}

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(launchParams testutil.TestParams) error {
		// onAppReady: Landing will appear in logcat after the game is fully loaded.
		if err := launchParams.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`))); err != nil {
			return errors.Wrap(err, "onAppReady was not found in LogCat")
		}

		testutil.PerformLogin(ctx, s, launchParams, loginHeuristics, func(loginParams testutil.TestParams) error {
			// onAppReady: AvatarExperienceLandingPage will appear in logcat after the game is fully loaded.
			if err := loginParams.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sAvatarExperienceLandingPage`))); err != nil {
				return errors.Wrap(err, "onAppReady was not found in LogCat")
			}

			// Save the metric in crosbolt.
			loadTime := time.Now().Sub(loginParams.ActivityStartTime)
			perfValues := perf.NewValues()
			perfValues.Set(testutil.LoginTimePerfMetric(), loadTime.Seconds())
			return perfValues.Save(s.OutDir())
		})
	})
}
