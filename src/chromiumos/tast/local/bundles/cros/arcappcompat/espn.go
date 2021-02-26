// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// ClamshellTests are placed here.
var clamshellTestsForESPN = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForESPN},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForESPN = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForESPN},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ESPN,
		Desc:         "Functional test for ESPN that installs the app also verifies it is logged in and that the main page is open, checks ESPN correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("elm")),
		Params: []testing.Param{{
			Val:               clamshellTestsForESPN,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForESPN,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForESPN,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForESPN,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// ESPN test uses library for opting into the playstore and installing app.
// Checks  ESPN correctly changes the window states in both clamshell and touchview mode.
func ESPN(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.espn.score_center"
		appActivity = "com.espn.sportscenter.ui.EspnLaunchActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForESPN verifies app is logged in and
// verify app reached main activity page of the app.
func launchAppForESPN(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		allowText       = "ALLOW"
		signUpLaterText = "Sign Up Later"
		skipText        = "Skip"
		finishText      = "Finish"
		okText          = "OK"
		homeID          = "com.espn.score_center:id/largeLabel"
	)

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	signUpButton := d.Object(ui.Text(signUpLaterText))
	if err := signUpButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Sign up button doesn't exist: ", err)
	} else if err := signUpButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign up button: ", err)
	}

	// Click on skip button.
	skipButton := d.Object(ui.Text(skipText))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("skip button doesn't exists: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip button: ", err)
	}

	// Click on finish button.
	finishButton := d.Object(ui.Text(finishText))
	if err := finishButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("finish button doesn't exists: ", err)
	} else if err := finishButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on finish button: ", err)
	}

	// Click on ok button.
	okButton := d.Object(ui.Text(okText))
	if err := okButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("ok button doesn't exists: ", err)
	} else if err := okButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on ok button: ", err)
	}

	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
