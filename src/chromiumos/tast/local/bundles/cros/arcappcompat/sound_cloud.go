// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForSoundCloud launches SoundCloud in clamshell mode.
var clamshellLaunchForSoundCloud = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForSoundCloud},
}

// touchviewLaunchForSoundCloud launches SoundCloud in tablet mode.
var touchviewLaunchForSoundCloud = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForSoundCloud},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     SoundCloud,
		Desc:     "Functional test for SoundCloud that installs the app also verifies it is logged in and that the main page is open, checks SoundCloud correctly changes the window state in both clamshell and touchview mode",
		Contacts: []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		// Disabled this test as soundcloud is migrated to PWA app.
		//Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForSoundCloud,
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
				Tests:      touchviewLaunchForSoundCloud,
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
				Tests:      clamshellLaunchForSoundCloud,
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
				Tests:      touchviewLaunchForSoundCloud,
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
			"arcappcompat.SoundCloud.emailid", "arcappcompat.SoundCloud.password"},
	})
}

// SoundCloud test uses library for opting into the playstore and installing app.
// Checks SoundCloud correctly changes the window states in both clamshell and touchview mode.
func SoundCloud(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.soundcloud.android"
		appActivity = ".launcher.LauncherActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForSoundCloud verifies SoundCloud is logged in and
// verify SoundCloud reached main activity page of the app.
func launchAppForSoundCloud(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueText             = "Continue"
		enterEmailAddressID      = "com.soundcloud.android:id/emailInputText"
		notNowID                 = "android:id/autofill_save_no"
		iHaveAnAccountButtonText = "I already have an account"
		passwordID               = "com.soundcloud.android:id/passwordInputText"
		passwordText             = "Password"
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

	emailAddress := s.RequiredVar("arcappcompat.SoundCloud.emailid")
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

	password := s.RequiredVar("arcappcompat.SoundCloud.password")
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

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
