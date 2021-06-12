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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForTwitch launches Twitch in clamshell mode.
var clamshellLaunchForTwitch = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForTwitch},
}

// touchviewLaunchForTwitch launches Twitch in tablet mode.
var touchviewLaunchForTwitch = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForTwitch},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Twitch,
		Desc:         "Functional test for Twitch that installs the app also verifies it is logged in and that the main page is open, checks Twitch correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForTwitch,
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
				Tests:      touchviewLaunchForTwitch,
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
				Tests:      clamshellLaunchForTwitch,
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
				Tests:      touchviewLaunchForTwitch,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Twitch.username", "arcappcompat.Twitch.password"},
	})
}

// Twitch test uses library for opting into the playstore and installing app.
// Checks Twitch correctly changes the window states in both clamshell and touchview mode.
func Twitch(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "tv.twitch.android.app"
		appActivity = ".core.LandingActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForTwitch verifies Twitch is logged in and
// verify Twitch reached main activity page of the app.
func launchAppForTwitch(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueButtonText    = "Continue"
		enterEmailAddressText = "Username"
		loginText             = "Log in"
		notNowID              = "android:id/autofill_save_no"
		neverButtonID         = "com.google.android.gms:id/credential_save_reject"
		passwordText          = "Password"
		homeiconID            = "tv.twitch.android.app:id/profile_pic_toolbar_image"
		resendCodeText        = "Resend code"
		shortTimeInterval     = 300 * time.Millisecond
	)

	// Click on login button.
	clickOnLoginButton := d.Object(ui.TextMatches("(?i)" + loginText))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("clickOnLoginButton doesn't exist: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	enterEmailAddress := d.Object(ui.TextMatches("(?i)" + enterEmailAddressText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("EnterEmailAddress does not exist: ", err)
	}

	// Enter email address.
	TwitchEmailID := s.RequiredVar("arcappcompat.Twitch.username")
	if err := kb.Type(ctx, TwitchEmailID); err != nil {
		s.Fatal("Failed to enter enterEmail: ", err)
	}
	s.Log("Entered enterEmail")

	enterPassword := d.Object(ui.TextMatches("(?i)" + passwordText))
	if err := enterPassword.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("EnterPassword does not exist: ", err)
	}

	// Press tab to click on password field.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
	}

	password := s.RequiredVar("arcappcompat.Twitch.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on Login button.
	loginButton := d.Object(ui.TextMatches("(?i)" + loginText))
	if err := loginButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("Login Button doesn't exist: ", err)
	} else {
		s.Log("Login Button does exist")
		// Press tab thrice to click on select login button.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		} else {
			s.Log("Entered KEYCODE_TAB")
			d.WaitForIdle(ctx, shortTimeInterval)
		}

		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		} else {
			s.Log("Entered KEYCODE_TAB")
			d.WaitForIdle(ctx, shortTimeInterval)
		}

		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		} else {
			s.Log("Entered KEYCODE_TAB")
			d.WaitForIdle(ctx, shortTimeInterval)
		}

		// Press enter to click on login button.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	}

	// Check for OTP
	checkForOTP := d.Object(ui.TextMatches("(?i)" + resendCodeText))
	if err := checkForOTP.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("checkForOTP doesn't exist: ", err)
	} else {
		s.Log("checkForOTP does exist")
		return
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

	// Click on continue button.
	continueButton := d.Object(ui.TextMatches("(?i)" + continueButtonText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
