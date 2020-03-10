// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForHulu = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForHulu},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForHulu = []testutil.TestSuite{
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
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForHulu,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedAppCompat(),
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForHulu,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForHulu,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForHulu,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               arc.VMBootedInTabletModeAppCompat(),
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Hulu test uses library for opting into the playstore and installing app.
// Checks Hulu correctly changes the window states in both clamshell and touchview mode.
func Hulu(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.hulu.plus"
		appActivity = "com.hulu.features.splash.SplashActivity"
	)

	// Step up chrome on Chromebook.
	cr, tconn, a, d := testutil.SetUpDevice(ctx, s, appPkgName, appActivity)

	testSet := s.Param().(testutil.TestParams)
	// Run the different test cases.
	for idx, test := range testSet.Tests {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			defer func() {
				if s.HasError() {
					path := fmt.Sprintf("%s/screenshot-arcappcompat-failed-test-%d.png", s.OutDir(), idx)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
				}
			}()
			// Launch the app.
			act, err := arc.NewActivity(a, appPkgName, appActivity)
			if err != nil {
				s.Fatal("Failed to create new app activity: ", err)
			}
			s.Log("Created new app activity")

			defer act.Close()
			if err := act.Start(ctx); err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			s.Log("App launched successfully")

			defer act.Stop(ctx)

			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForHulu verifies Hulu is logged in and
// verify Hulu reached main activity page of the app.
func launchAppForHulu(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	const (
		loginID         = "com.hulu.plus:id/secondary_button"
		enterEmailID    = "com.hulu.plus:id/email"
		enterPasswordID = "com.hulu.plus:id/password"
		loginButtonID   = "com.hulu.plus:id/login_button"
		userText        = "Rohit"
		homeIconID      = "com.hulu.plus:id/menu_home"
		huluEmailID     = "rohitbm@google.com"
		huluPassword    = "SS0me1kn0ws"
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
		s.Log("EnterEmailAddress does exist")
	}
	// Enter email address.
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if enterEmailAddress.SetText(ctx, huluEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter Password.
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if enterPassword.SetText(ctx, huluPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	testSet := s.Param().(testutil.TestParams)
	deviceMode := "clamshell"
	if testSet.TabletMode {
		deviceMode = "tablet"
		// Press back to make login button visible.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Doesn't enter KEYCODE_BACK: ", err)
		} else {
			s.Log("Entered KEYCODE_BACK")
		}
	} else {
		s.Logf("device %v mode", deviceMode)
	}

	// Click on Login button again.
	clickOnLoginButton := d.Object(ui.ID(loginButtonID))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Login Button doesn't exist: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
	}

	// Select User.
	selectUser := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(userText))
	if err := selectUser.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("SelectUser doesn't exist: ", err)
	} else if err := selectUser.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectUser: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeIconID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("SelectUser doesn't exist: ", err)
	}

	defer signOutOfHulu(ctx, s, a, d, appPkgName, appActivity)

}

// signOutOfHulu verifies app is signed out.
func signOutOfHulu(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	const (
		accountIconID    = "com.hulu.plus:id/menu_account"
		logOutOfHuluText = "Log out of Hulu"
	)

	// Click on account icon.
	accountIcon := d.Object(ui.ID(accountIconID))
	if err := accountIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("AccountIcon doesn't exist: ", err)
	} else if err := accountIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on accountIcon: ", err)
	}

	// Click on log out of Hulu.
	logOutOfHulu := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(logOutOfHuluText))
	if err := logOutOfHulu.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LogOutOfHulu doesn't exist: ", err)
	} else if err := logOutOfHulu.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfHulu: ", err)
	}
}
