// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForHulu = []testutil.TestSuite{
	{"Launch app in Clamshell", launchAppForHulu},
	{"Clamshell: Fullscreen app", testutil.ClamshellFullscreenApp},
	{"Clamshell: Minimise and Restore", testutil.MinimizeRestoreApp},
	{"Clamshell: Resize window", testutil.ClamshellResizeWindow},
	{"Clamshell: Reopen app", testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForHulu = []testutil.TestSuite{
	{"Launch app in Touchview", launchAppForHulu},
	{"Touchview: Minimise and Restore", testutil.MinimizeRestoreApp},
	{"Touchview: Reopen app", testutil.ReOpenWindow},
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
				false,
				clamshellTestsForHulu,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedAppCompat(),
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				true,
				touchviewTestsForHulu,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				false,
				clamshellTestsForHulu,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				true,
				touchviewTestsForHulu,
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

		openButtonRegex = "Open|OPEN"
	)

	// Setup Chrome.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()
	s.Log("Enable showing ANRs")
	if err := a.Command(ctx, "settings", "put", "secure", "anr_show_background", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable showing ANRs: ", err)
	}
	s.Log("Enable crash dialog")
	if err := a.Command(ctx, "settings", "put", "secure", "show_first_crash_dialog_dev_option", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable crash dialog: ", err)
	}

	s.Log("Installing app")
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	s.Log("Launch the app")
	// Click on open button.
	openButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches(openButtonRegex))
	must(openButton.WaitForExists(ctx, testutil.LongUITimeout))
	// Open button exist and click.
	must(openButton.Click(ctx))

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
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForHulu verifies Hulu is logged in and
// verify Hulu reached main activity page of the app.
func launchAppForHulu(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
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
	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

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
		s.Log("enterEmailAddress doesn't exist: ", err)
	} else {
		s.Log("enterEmailAddress does exist ")
	}
	// Enter email address.
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("enterEmailAddress doesn't exist: ", err)
	} else {
		must(enterEmailAddress.Click(ctx))
		must(enterEmailAddress.SetText(ctx, huluEmailID))
	}

	// Enter Password.
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("enterPassword doesn't exist: ", err)
	} else {
		must(enterPassword.Click(ctx))
		must(enterPassword.SetText(ctx, huluPassword))
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
		s.Log("login Button doesn't exist: ", err)
	} else {
		must(clickOnLoginButton.Click(ctx))
	}

	// Select User.
	selectUser := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(userText))
	if err := selectUser.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("selectUser doesn't exist: ", err)
	} else {
		must(selectUser.Click(ctx))
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeIconID))
	must(homeIcon.WaitForExists(ctx, testutil.LongUITimeout))

	defer signOutOfHulu(ctx, s, a, d, appPkgName, appActivity)

}

// signOutOfHulu verifies app is signed out.
func signOutOfHulu(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	const (
		accountIconID    = "com.hulu.plus:id/menu_account"
		logOutOfHuluText = "Log out of Hulu"
	)
	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}
	// Click on account icon.
	accountIcon := d.Object(ui.ID(accountIconID))
	if err := accountIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("accountIcon doesn't exist: ", err)
	} else {
		must(accountIcon.Click(ctx))
	}

	// Click on log out of Hulu.
	logOutOfHulu := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(logOutOfHuluText))
	if err := logOutOfHulu.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("logOutOfHulu doesn't exist: ", err)
	} else {
		must(logOutOfHulu.Click(ctx))
	}
}
