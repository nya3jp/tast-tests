// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForKhanacademyKids launches KhanacademyKids in clamshell mode.
var clamshellLaunchForKhanacademyKids = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForKhanacademyKids, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForKhanacademyKids launches KhanacademyKids in tablet mode.
var touchviewLaunchForKhanacademyKids = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForKhanacademyKids, Timeout: testutil.LaunchTestCaseTimeout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         KhanacademyKids,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for KhanacademyKids that installs the app also verifies it is logged in and that the main page is open, checks KhanacademyKids correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_top_apps"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForKhanacademyKids,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForKhanacademyKids,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForKhanacademyKids,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForKhanacademyKids,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForKhanacademyKids,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForKhanacademyKids,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForKhanacademyKids,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForKhanacademyKids,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 20 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.KhanacademyKids.emailid", "arcappcompat.KhanacademyKids.password"},
	})
}

// KhanacademyKids test uses library for opting into the playstore and installing app.
// Checks KhanacademyKids correctly changes the window states in both clamshell and touchview mode.
func KhanacademyKids(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "org.khankids.android"
		appActivity = ".MainActivity"
	)

	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForKhanacademyKids verifies KhanacademyKids is logged in and
// verify KhanacademyKids reached main activity page of the app.
func launchAppForKhanacademyKids(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		userEmailText     = "@gmail.com"
		editTextClassName = "android.widget.EditText"
		emailText         = "Email"
		okayText          = "Okay"
		passwordText      = "Password"
		textViewClassName = "android.widget.TextView"
		notNowID          = "android:id/autofill_save_no"
		neverButtonID     = "com.google.android.gms:id/credential_save_reject"
		nextButtonWord    = "Next"
		userProfileWord   = "apps"

		waitForActiveInputTime = time.Second * 10
	)
	// Click on ok button.
	okButton := d.Object(ui.ClassName(textViewClassName), ui.TextMatches("(?i)"+okayText))
	if err := okButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("ok button doesn't exist: ", err)
	} else if err := okButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on ok button: ", err)
	}
	// Click on allow button.
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Click on signin button.
	signInButton := uidetection.TextBlock([]string{"Sign", "in", "with", "Khan", "Kids", "account"})
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for sign in button",
		ud.WaitUntilExists(signInButton),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(signInButton),
	)(ctx); err != nil {
		s.Fatal("Failed to find sign in button: ", err)
	}

	//  Check if user gmail account exist.
	userEmail := d.Object(ui.TextContains(userEmailText))
	if err := userEmail.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("userEmail doesn't exist: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		s.Fatal("Failed to press back: ", err)
	}

	// Click on enter email address.
	enterEmailAddress := d.Object(ui.ClassName(editTextClassName), ui.TextMatches("(?i)"+emailText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("enterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	// Click on emailid text field until the emailid text field is focused.
	testutil.ClickUntilFocused(ctx, s, tconn, a, d, enterEmailAddress)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.KhanacademyKids.emailid")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Enter password.
	enterPassword := d.Object(ui.ClassName(editTextClassName), ui.TextMatches("(?i)"+passwordText))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterPassword does not exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}
	// Click on password text field until the password text field is focused.
	testutil.ClickUntilFocused(ctx, s, tconn, a, d, enterPassword)

	password := s.RequiredVar("arcappcompat.KhanacademyKids.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on next button.
	nextButton := uidetection.Word(nextButtonWord)
	ud = uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for next button",
		ud.WaitUntilExists(nextButton),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(uidetection.Word(nextButtonWord)),
	)(ctx); err != nil {
		s.Fatal("Failed to find next button: ", err)
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

	// Check for user profile icon.
	userProfileIcon := uidetection.Word(userProfileWord)
	ud = uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for user profile icon",
		ud.WaitUntilExists(userProfileIcon),
		action.Sleep(waitForActiveInputTime),
	)(ctx); err != nil {
		s.Fatal("Failed to find user profile icon: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
