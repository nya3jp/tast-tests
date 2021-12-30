// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"math"
	"regexp"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cpu"
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
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

// loginHeuristics stores data about the location of various fields.
type loginHeuristicsFields struct {
	FirstLoginButton  float64
	UsernameField     float64
	UsernameString    string
	PasswordField     float64
	PasswordString    string
	SecondLoginButton float64
	Bounds            coords.Rect
}

// cleanupOnErrorTime reserves time for cleanup in case of an error.
const cleanupOnErrorTime = time.Second * 30

// getCoords returns an approximate pixel location given an activity and a
// heuristic.
func getCoords(bounds coords.Rect, heuristic, scaleFactor float64) coords.Point {
	relativeWidth := 0.5 / scaleFactor
	relativeHeight := heuristic / scaleFactor
	return coords.Point{int(float64(bounds.Width) * relativeWidth), int(float64(bounds.Height) * relativeHeight)}
}

// loginTimePerfMetric returns a standard metric that login time can be saved in.
func loginTimePerfMetric() perf.Metric {
	return perf.Metric{
		Name:      "loginTime",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}
}

// displayScaleFactor returns the scale factor for the current display.
func displayScaleFactor(ctx context.Context, tconn *chrome.TestConn) (float64, error) {
	// Find the ratio to convert coordinates in the screenshot to those in the screen.
	screens, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return 1.0, errors.Wrap(err, "failed to get the display info")
	}

	scaleFactor, err := screens[0].GetEffectiveDeviceScaleFactor()
	if err != nil {
		return 1.0, errors.Wrap(err, "failed to get the device scale factor")
	}

	// Make sure the scale factor is neither 0 nor NaN.
	if math.IsNaN(scaleFactor) || math.Abs(scaleFactor) < 1e-10 {
		return 1.0, errors.Errorf("invalid device scale factor: %f", scaleFactor)
	}

	return scaleFactor, nil
}

// performLogin assumes a fully launched activity, performs a login, and defers to the caller to perform a test.
func performLogin(ctx context.Context, s *testing.State, params testutil.TestParams, loginHeuristics loginHeuristicsFields, testFunc testutil.PerformTestFunc) {
	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupOnErrorTime)
	defer cancel()

	// Pull out the common values.
	cr := s.PreValue().(arc.PreData).Chrome

	// Wait for the CPU to idle before performing the test.
	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
		s.Fatal("Failed to wait until CPU is cooled down: ", err)
	}

	// Take screenshot on failure.
	defer func(ctx context.Context) {
		if s.HasError() {
			testutil.CaptureScreenshot(ctx, s, cr, "failed-login-test.png")
		}
	}(cleanupCtx)

	// Start up keyboard.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()

	// Derive activity and test connection.
	act := params.Activity
	tconn := params.TestConn

	scaleFactor, err := displayScaleFactor(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get scale factor: ", err)
	}

	// Click screen anywhere.
	if err = mouse.Click(tconn, coords.Point{0, 0}, mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click first login button: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Locate/click on login button.
	if err = mouse.Click(tconn, getCoords(loginHeuristics.Bounds, loginHeuristics.FirstLoginButton, scaleFactor), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click first login button: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Locate/click username field, and type username.
	if err = mouse.Click(tconn, getCoords(loginHeuristics.Bounds, loginHeuristics.UsernameField, scaleFactor), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click username field: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err = ew.Type(ctx, loginHeuristics.UsernameString); err != nil {
		s.Fatal("Failed to write username: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Locate/click password field, and enter.
	if err = mouse.Click(tconn, getCoords(loginHeuristics.Bounds, loginHeuristics.PasswordField, scaleFactor), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click password field: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err = ew.Type(ctx, loginHeuristics.PasswordString); err != nil {
		s.Fatal("Failed to write password: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Locate login button, start timer, and click login button.
	if err = mouse.Click(tconn, getCoords(loginHeuristics.Bounds, loginHeuristics.SecondLoginButton, scaleFactor), mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click second login button: ", err)
	}
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Reassign start time.
	startTime := time.Now()

	// Defer to the caller to determine when the game is logged in.
	if err := testFunc(testutil.TestParams{
		TestConn:          tconn,
		Arc:               params.Arc,
		Device:            params.Device,
		AppPkgName:        params.AppPkgName,
		AppActivityName:   params.AppActivityName,
		Activity:          act,
		ActivityStartTime: startTime,
	}); err != nil {
		s.Fatal("Failed to perform test (post login): ", err)
	}
}

func RobloxLogin(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
	)

	// Hard coded login field heuristics.
	const (
		FirstLoginButton  = 0.53
		UsernameField     = 0.46
		UsernameString    = "testaccountgoogle12"
		PasswordField     = 0.53
		PasswordString    = "username1"
		SecondLoginButton = 0.6
	)

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(launchParams testutil.TestParams) error {
		// Pull out screen bounds first
		bounds, err := launchParams.Activity.SurfaceBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get surface bounds: ", err)
		}
		if bounds.Width <= 0 || bounds.Height <= 0 {
			s.Fatal("Bounds should be positive: ", err)
		}

		loginHeuristics := loginHeuristicsFields{FirstLoginButton, UsernameField, UsernameString, PasswordField, PasswordString, SecondLoginButton, bounds}

		// onAppReady: Landing will appear in logcat after the game is fully loaded.
		launchParams.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`)))

		performLogin(ctx, s, launchParams, loginHeuristics, func(loginParams testutil.TestParams) error {
			// onAppReady: AvatarExperienceLandingPage will appear in logcat after the game is fully loaded.
			if err := loginParams.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sAvatarExperienceLandingPage`))); err != nil {
				return errors.Wrap(err, "onAppReady was not found in LogCat")
			}

			// Save the metric in crosbolt.
			loadTime := time.Now().Sub(loginParams.ActivityStartTime)
			perfValues := perf.NewValues()
			perfValues.Set(loginTimePerfMetric(), loadTime.Seconds())

			if err := perfValues.Save(s.OutDir()); err != nil {
				return errors.Wrap(err, "failed to save performance values")
			}

			return nil
		})

		return nil
	})
}
