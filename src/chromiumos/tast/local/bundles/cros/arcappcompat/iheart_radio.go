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

// clamshellLaunchForIHeartRadio launches IHeartRadio in clamshell mode.
var clamshellLaunchForIHeartRadio = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForIHeartRadio},
}

// touchviewLaunchForIHeartRadio launches IHeartRadio in tablet mode.
var touchviewLaunchForIHeartRadio = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForIHeartRadio},
}

// clamshellAppSpecificTestsForIHeartRadio are placed here.
var clamshellAppSpecificTestsForIHeartRadio = []testutil.TestSuite{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForIHeartRadio are placed here.
var touchviewAppSpecificTestsForIHeartRadio = []testutil.TestSuite{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         IHeartRadio,
		Desc:         "Functional test for IHeartRadio that installs the app also verifies it is logged in and that the main page is open, checks IHeartRadio correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForIHeartRadio,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForIHeartRadio,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForIHeartRadio,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForIHeartRadio,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForIHeartRadio,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForIHeartRadio,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForIHeartRadio,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForIHeartRadio,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.IHeartRadio.emailid", "arcappcompat.IHeartRadio.password"},
	})
}

// IHeartRadio test uses library for opting into the playstore and installing app.
// Checks IHeartRadio correctly changes the window states in both clamshell and touchview mode.
func IHeartRadio(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.clearchannel.iheartradio.controller"
		appActivity = ".activities.NavDrawerActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForIHeartRadio verifies IHeartRadio is logged in and
// verify IHeartRadio reached main activity page of the app.
func launchAppForIHeartRadio(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInID        = "com.clearchannel.iheartradio.controller:id/login_button"
		enterEmailID    = "com.clearchannel.iheartradio.controller:id/email"
		enterPasswordID = "com.clearchannel.iheartradio.controller:id/password"
		logInID         = "com.clearchannel.iheartradio.controller:id/email_login"
		notNowText      = "NOT NOW"
		skipText        = "Skip"
		notNowID        = "android:id/autofill_save_no"
		neverButtonID   = "com.google.android.gms:id/credential_save_reject"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign in button: ", err)
	}

	// Check and click email address.
	IHeartRadioEmailID := s.RequiredVar("arcappcompat.IHeartRadio.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, IHeartRadioEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	IHeartRadioPassword := s.RequiredVar("arcappcompat.IHeartRadio.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterPassword doesn't exist: ", err)
	} else if err := enterPassword.SetText(ctx, IHeartRadioPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on login button.
	logIntButton := d.Object(ui.ID(logInID))
	if err := logIntButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("login  button doesn't exist: ", err)
	} else if err := logIntButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on login  button: ", err)
	}

	// Click on not now button.
	notNowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(notNowText))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on never button.
	neverButton := d.Object(ui.ID(neverButtonID))
	if err := neverButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Never Button doesn't exist: ", err)
	} else if err := neverButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on neverButton: ", err)
	}

	// Click on no thanks button.
	clickOnNoThanksButton := d.Object(ui.ID(notNowID))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}

	// Click on skip button.
	skipButton := d.Object(ui.Text(skipText))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("skip button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
