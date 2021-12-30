// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RobloxLogin,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Captures login metrics for Roblox",
		Contacts:     []string{"davidwelling@google.com", "pjlee@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		// TODO(b/206442649): Remove after initial testing is complete.
		HardwareDeps: hwdep.D(hwdep.Model("eve", "volteer", "zork")),
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

// sleepTime reserves time to wait between peripheral interactions.
const sleepTime = time.Second * 15

// performLogin assumes a fully launched Roblox activity, performs a login, returning an errors that occur, and uploads the login time metric.
func performLogin(ctx context.Context, s *testing.State, params testutil.TestParams, bounds coords.Rect) error {
	// Hard coded heuristics for Roblox login.Roblox
	const (
		FirstLoginButton  = 0.53
		UsernameField     = 0.46
		PasswordField     = 0.53
		SecondLoginButton = 0.6
	)

	// Start up keyboard.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer ew.Close()

	// Derive test connection.
	tconn := params.TestConn

	// Get Username and Password for Roblox.
	username := s.RequiredVar("arcappgameperf.roblox_username")
	password := s.RequiredVar("arcappgameperf.roblox_password")

	// Click screen anywhere.
	// TODO(b/215396708): Find solution for additional mouse click.
	if err = mouse.Click(tconn, coords.Point{0, 0}, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click first login button")
	}
	if err = testing.Sleep(ctx, sleepTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Locate/click on login button.
	loginCoords1, err := testutil.GetCoords(ctx, tconn, bounds, 0.5, FirstLoginButton)
	if err != nil {
		return errors.Wrap(err, "failed to get first login coordinates")
	}
	if err = mouse.Click(tconn, loginCoords1, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click first login button")
	}
	if err = testing.Sleep(ctx, sleepTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Locate/click username field, and type username.
	usernameCoords, err := testutil.GetCoords(ctx, tconn, bounds, 0.5, UsernameField)
	if err != nil {
		return errors.Wrap(err, "failed to get username coordinates")
	}
	if err = mouse.Click(tconn, usernameCoords, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click username field")
	}
	if err = testing.Sleep(ctx, sleepTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	if err = ew.Type(ctx, username); err != nil {
		return errors.Wrap(err, "failed to write username")
	}
	if err = testing.Sleep(ctx, sleepTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Locate/click password field, and enter.
	passwordCoords, err := testutil.GetCoords(ctx, tconn, bounds, 0.5, PasswordField)
	if err != nil {
		return errors.Wrap(err, "failed to get password coordinates")
	}
	if err = mouse.Click(tconn, passwordCoords, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click password field")
	}
	if err = testing.Sleep(ctx, sleepTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	if err = ew.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to write password")
	}
	if err = testing.Sleep(ctx, sleepTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Locate login button, start timer, and click login button.
	loginCoords2, err := testutil.GetCoords(ctx, tconn, bounds, 0.5, SecondLoginButton)
	if err != nil {
		return errors.Wrap(err, "failed to get password coordinates")
	}
	if err = mouse.Click(tconn, loginCoords2, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click second login button")
	}
	if err = testing.Sleep(ctx, sleepTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Reassign start time.
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
}

func RobloxLogin(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
	)

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(launchParams testutil.TestParams) error {
		// Pull out screen bounds first
		bounds, err := launchParams.Activity.SurfaceBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get surface bounds")
		}
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return errors.New("Bounds should be positive")
		}

		// onAppReady: Landing will appear in logcat after the game is fully loaded.
		launchParams.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`)))

		return performLogin(ctx, s, launchParams, bounds)
	})
}
