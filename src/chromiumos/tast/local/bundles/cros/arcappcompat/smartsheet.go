// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForSmartsheet launches Smartsheet in clamshell mode.
var clamshellLaunchForSmartsheet = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSmartsheet},
}

// touchviewLaunchForSmartsheet launches Smartsheet in tablet mode.
var touchviewLaunchForSmartsheet = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSmartsheet},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Smartsheet,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Smartsheet that installs the app also verifies it is logged in and that the main page is open, checks Smartsheet correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForSmartsheet,
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
				LaunchTests: touchviewLaunchForSmartsheet,
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
				LaunchTests: clamshellLaunchForSmartsheet,
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
				LaunchTests: touchviewLaunchForSmartsheet,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.Smartsheet.emailid", "arcappcompat.Smartsheet.password"},
	})
}

// Smartsheet test uses library for opting into the playstore and installing app.
// Checks Smartsheet correctly changes the window states in both clamshell and touchview mode.
func Smartsheet(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.smartsheet.android"
		appActivity = ".activity.launcher.LauncherActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForSmartsheet verifies Smartsheet is logged in and
// verify Smartsheet reached main activity page of the app.
func launchAppForSmartsheet(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueText             = "CONTINUE"
		enterEmailAddressID      = "com.smartsheet.android:id/email"
		gotItID                  = "com.smartsheet.android:id/button_got_it"
		gotItBtnID               = "com.smartsheet.android:id/gotItButton"
		letUsConfirmText         = "Let's confirm it's you"
		navigateClassName        = "android.widget.ImageButton"
		navigateDes              = "Navigate up"
		notNowID                 = "android:id/autofill_save_no"
		iHaveAnAccountButtonText = "I already have an account"
		okID                     = "com.smartsheet.android:id/buttonOk"
		passwordID               = "com.smartsheet.android:id/password"
		passwordText             = "Password"
		selectText               = "SELECT"
		skipID                   = "com.smartsheet.android:id/onboarding_skip"
		signInClassName          = "android.widget.Button"
		signInText               = "LOG IN"
		trySmartsheetForFreeText = "Try Smartsheet for free"
		homeID                   = "com.smartsheet.android:id/add_new_button"
	)

	// Click on I have an account button.
	clickOnIHavaAnAccountButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+iHaveAnAccountButtonText))
	if err := clickOnIHavaAnAccountButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("clickOnIHavaAnAccountButton doesn't exists: ", err)
	} else if err := clickOnIHavaAnAccountButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnIHavaAnAccountButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exists: ", err)
	}

	// Click on email address text field until the email address text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if emailAddressFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
			return errors.New("emailAddress text field not focused yet")
		} else if !emailAddressFocused {
			enterEmailAddress.Click(ctx)
			return errors.New("emailAddress text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus emailAddress: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	emailAddress := s.RequiredVar("arcappcompat.Smartsheet.emailid")
	if err := kb.Type(ctx, emailAddress); err != nil {
		s.Fatal("Failed to enter emailAddress: ", err)
	}
	// Press enter to select email address.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	} else {
		s.Log("Entered KEYCODE_ENTER")
	}
	s.Log("Entered EmailAddress")

	// Click on Sign in button.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signInText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("SignInButton doesn't exists: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Log("Failed to click on signInButton: ", err)
	}

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("EnterPassword doesn't exists: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	// Click on password text field until the password text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
			return errors.New("password text field not focused yet")
		} else if !pwdFocused {
			enterPassword.Click(ctx)
			return errors.New("password text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.Smartsheet.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on continue button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on notNow button.
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on got it button.
	gotItButton := d.Object(ui.ID(gotItID))
	if err := gotItButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("gotItButton doesn't exists: ", err)
	} else if err := gotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on gotItButton: ", err)

	}

	// Click on ok button.
	okButton := d.Object(ui.ID(okID))
	if err := okButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("okButton doesn't exists: ", err)
	} else if err := okButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on okButton: ", err)

	}

	// Click on got it button.
	gotItButton = d.Object(ui.ID(gotItBtnID))
	if err := gotItButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("gotItButton doesn't exists: ", err)
	} else if err := gotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on gotItButton: ", err)

	}

	// Click on skip button.
	skipButton := d.Object(ui.ID(skipID))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("skipButton doesn't exists: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)

	}

	// Click on select button.
	selectButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+selectText))
	if err := selectButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("selectButton doesn't exists: ", err)
	} else {
		s.Log("selectButton does exists")
		// Click on navigate button.
		navigateButton := d.Object(ui.ClassName(navigateClassName), ui.Description(navigateDes))
		if err := navigateButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Log("navigateButton doesn't exists: ", err)
		} else if err := navigateButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on navigateButton: ", err)

		}
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
