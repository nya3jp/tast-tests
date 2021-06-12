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

// clamshellLaunchForESPN launches ESPN in clamshell mode.
var clamshellLaunchForESPN = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForESPN},
}

// touchviewLaunchForESPN launches ESPN in tablet mode.
var touchviewLaunchForESPN = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForESPN},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ESPN,
		Desc:         "Functional test for ESPN that installs the app also verifies it is logged in and that the main page is open, checks ESPN correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("elm")),
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForESPN,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForESPN,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForESPN,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForESPN,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// ESPN test uses library for opting into the playstore and installing app.
// Checks  ESPN correctly changes the window states in both clamshell and touchview mode.
func ESPN(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.espn.score_center"
		appActivity = "com.espn.sportscenter.ui.EspnLaunchActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
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
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
