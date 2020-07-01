// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
			"arcappcompat.Hulu.emailid", "arcappcompat.Hulu.password", "arcappcompat.Hulu.username"},
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
		loginID         = "com.hulu.plus:id/secondary_button"
		enterEmailID    = "com.hulu.plus:id/email"
		enterPasswordID = "com.hulu.plus:id/password"
		loginButtonID   = "com.hulu.plus:id/login_button"
		homeIconID      = "com.hulu.plus:id/menu_home"
	)

	loginButton := d.Object(ui.ID(loginID))
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	// Keep clicking login Button until enterEmailAddress exist in the home page.
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

	// Select User.
	userText := s.RequiredVar("arcappcompat.Hulu.username")
	selectUser := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(userText))
	if err := selectUser.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SelectUser doesn't exist: ", err)
	} else if err := selectUser.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectUser: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeIconID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("SelectUser doesn't exist: ", err)
	}

	signOutOfHulu(ctx, s, a, d, appPkgName, appActivity)

}

// signOutOfHulu verifies app is signed out.
func signOutOfHulu(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		accountIconID    = "com.hulu.plus:id/menu_account"
		logOutOfHuluText = "Log out of Hulu"
	)

	// Click on account icon.
	accountIcon := d.Object(ui.ID(accountIconID))
	if err := accountIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("AccountIcon doesn't exist: ", err)
	} else if err := accountIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on accountIcon: ", err)
	}

	// Click on log out of Hulu.
	logOutOfHulu := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(logOutOfHuluText))
	if err := logOutOfHulu.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogOutOfHulu doesn't exist: ", err)
	} else if err := logOutOfHulu.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfHulu: ", err)
	}
}
