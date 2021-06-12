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

// clamshellLaunchForBandlab launches Bandlab in clamshell mode.
var clamshellLaunchForBandlab = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForBandlab},
}

// touchviewLaunchForBandlab launches Bandlab in tablet mode.
var touchviewLaunchForBandlab = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForBandlab},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Bandlab,
		Desc:         "Functional test for Bandlab that installs the app also verifies it is logged in and that the main page is open, checks Bandlab correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForBandlab,
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
				Tests:      touchviewLaunchForBandlab,
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
				Tests:      clamshellLaunchForBandlab,
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
				Tests:      touchviewLaunchForBandlab,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Bandlab.emailid", "arcappcompat.Bandlab.password"},
	})
}

// Bandlab test uses library for opting into the playstore and installing app.
// Checks Bandlab correctly changes the window states in both clamshell and touchview mode.
func Bandlab(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.bandlab.bandlab"
		appActivity = ".core.activity.navigation.NavigationActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForBandlab verifies Bandlab is logged in and
// verify Bandlab reached main activity page of the app.
func launchAppForBandlab(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		logInID          = "com.bandlab.bandlab:id/join_go_log_in"
		enterEmailID     = "com.bandlab.bandlab:id/login_user_name"
		enterPasswordID  = "com.bandlab.bandlab:id/login_password"
		logInText        = "Log In"
		logInButtonID    = "com.bandlab.bandlab:id/login_btn"
		neverButtonText  = "NEVER"
		noThanksButtonID = "android:id/autofill_save_no"
		createIconID     = "com.bandlab.bandlab:id/add"
		notNowText       = "Not Now"
		createIconDesc   = "Create"
	)

	// Click on log in button.
	logInButton := d.Object(ui.ID(logInID))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Log in button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Log in button: ", err)
	}

	// Enter email address.
	BandlabEmailID := s.RequiredVar("arcappcompat.Bandlab.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, BandlabEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	BandlabPassword := s.RequiredVar("arcappcompat.Bandlab.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, BandlabPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on log in button
	logInBtn := d.Object(ui.ID(logInButtonID), ui.Text(logInText))
	if err := logInBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogIn button doesn't exist: ", err)
	} else if err := logInBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on LogIn button: ", err)
	}

	// Click on no thanks or never button
	noThanksButton := d.Object(ui.ID(noThanksButtonID))
	neverButton := d.Object(ui.Text(neverButtonText))
	if err := noThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("noThanks button doesn't exist: ", err)
		neverButton.Click(ctx)
	} else if err := noThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on noThanks button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
