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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForHulu = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForHulu},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForHulu = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForHulu},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Hulu,
		Desc:         "Functional test for Hulu that installs the app also verifies it is logged in and that the main page is open, checks Hulu correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForHulu,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForHulu,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForHulu,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForHulu,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Hulu.emailid", "arcappcompat.Hulu.password"},
	})
}

// Hulu test uses library for opting into the playstore and installing app.
// Checks Hulu correctly changes the window states in both clamshell and touchview mode.
func Hulu(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.hulu.plus"
		appActivity = "com.hulu.features.splash.SplashActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForHulu verifies Hulu is logged in and
// verify Hulu reached main activity page of the app.
func launchAppForHulu(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText           = "ALLOW"
		continueText              = "CONTINUE"
		enableLocationServiceText = "ENABLE LOCATION SERVICES"
		loginText                 = "LOG IN|Log in"
		enterEmailID              = "com.hulu.plus:id/email"
		enterPasswordID           = "com.hulu.plus:id/password"
		loginButtonID             = "com.hulu.plus:id/login_button"
		notNowID                  = "android:id/autofill_save_no"
		neverButtonID             = "com.google.android.gms:id/credential_save_reject"
		homeIconID                = "com.hulu.plus:id/menu_home"
	)

	loginButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches(loginText))
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	// Click on login Button until enterEmailAddress exist in the home page.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := enterEmailAddress.Exists(ctx); err != nil {
			loginButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("EnterEmailAddress doesn't exist: ", err)
	} else {
		s.Log("EnterEmailAddress does exists")
	}
	// Enter email address.
	huluEmailID := s.RequiredVar("arcappcompat.Hulu.emailid")
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if enterEmailAddress.SetText(ctx, huluEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter Password.
	huluPassword := s.RequiredVar("arcappcompat.Hulu.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if enterPassword.SetText(ctx, huluPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	deviceMode := "clamshell"
	if tabletModeEnabled {
		deviceMode = "tablet"
		// Press back to make login button visible.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Failed to enter KEYCODE_BACK: ", err)
		} else {
			s.Log("Entered KEYCODE_BACK")
		}
	}
	s.Logf("device %v mode", deviceMode)

	// Click on Login button again.
	clickOnLoginButton := d.Object(ui.ID(loginButtonID))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Login Button doesn't exist: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
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

	// Click on enable location service.
	clickOnLocationServices := d.Object(ui.TextMatches("(?i)" + enableLocationServiceText))
	if err := clickOnLocationServices.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnLocationServices doesn't exist: ", err)
	} else if err := clickOnLocationServices.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLocationServices: ", err)
	}

	// Click on continue button.
	clickOncontinueButton := d.Object(ui.TextMatches("(?i)" + continueText))
	if err := clickOncontinueButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOncontinueButton doesn't exist: ", err)
	} else if err := clickOncontinueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOncontinueButton: ", err)
	}

	// Click on allow button to access device location.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}
	// Press back key to dismiss the pop up.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		s.Log("Failed to enter KEYCODE_BACk: ", err)
	} else {
		s.Log("Entered KEYCODE_BACK")
	}

	// Launch the hulu app.
	if err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "-c", "android.intent.category.LAUNCHER", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start Hulu app: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeIconID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("HomeIcon doesn't exist: ", err)
	} else {
		s.Log("HomeIcon does exist")
		signOutOfHulu(ctx, s, a, d, appPkgName, appActivity)
	}

}

// signOutOfHulu verifies app is signed out.
func signOutOfHulu(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		accountIconID    = "com.hulu.plus:id/menu_profile"
		logoutText       = "Log Out"
		logOutOfHuluText = "LOG OUT"
	)
	// Click on account icon.
	accountIcon := d.Object(ui.ID(accountIconID))
	if err := accountIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("AccountIcon doesn't exist: ", err)
	} else if err := accountIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on accountIcon: ", err)
	}
	// Click on logout button.
	logoutbutton := d.Object(ui.TextMatches("(?i)" + logoutText))
	if err := logoutbutton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("logoutbutton doesn't exist: ", err)
	} else if err := logoutbutton.Click(ctx); err != nil {
		s.Fatal("Failed to click on logoutbutton: ", err)
	}

	// Click on log out of Hulu.
	logOutOfHulu := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+logOutOfHuluText))
	if err := logOutOfHulu.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogOutOfHulu doesn't exist: ", err)
	} else if err := logOutOfHulu.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfHulu: ", err)
	}
}
