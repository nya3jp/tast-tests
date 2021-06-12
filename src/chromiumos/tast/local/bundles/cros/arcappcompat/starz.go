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

// clamshellLaunchForStarz launches Starz in clamshell mode.
var clamshellLaunchForStarz = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForStarz},
}

// touchviewLaunchForStarz launches Starz in tablet mode.
var touchviewLaunchForStarz = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForStarz},
}

// clamshellAppSpecificTestsForStarz are placed here.
var clamshellAppSpecificTestsForStarz = []testutil.TestSuite{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Clamshell: Signout app", Fn: signOutOfStarz},
}

// touchviewAppSpecificTestsForStarz are placed here.
var touchviewAppSpecificTestsForStarz = []testutil.TestSuite{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Touchview: Signout app", Fn: signOutOfStarz},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Starz,
		Desc:         "Functional test for Starz that installs the app also verifies it is logged in and that the main page is open, checks Starz correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForStarz,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForStarz,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForStarz,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForStarz,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForStarz,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForStarz,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForStarz,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForStarz,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Starz.emailid", "arcappcompat.Starz.password"},
	})
}

// Starz test uses library for opting into the playstore and installing app.
// Checks Starz correctly changes the window states in both clamshell and touchview mode.
func Starz(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.bydeluxe.d3.android.program.starz"
		appActivity = "com.starz.handheld.SplashActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForStarz verifies Starz is logged in and
// verify Starz reached main activity page of the app.
func launchAppForStarz(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		dismissButtonID     = "android:id/button2"
		enterEmailAddressID = "com.bydeluxe.d3.android.program.starz:id/email_et"
		loginButtonID       = "com.bydeluxe.d3.android.program.starz:id/login_link"
		loginID             = "com.bydeluxe.d3.android.program.starz:id/login_button"
		notNowID            = "android:id/autofill_save_no"
		passwordID          = "com.bydeluxe.d3.android.program.starz:id/password_et"
		homeID              = "com.bydeluxe.d3.android.program.starz:id/action_home"
	)

	// Check for login button.
	loginButton := d.Object(ui.ID(loginButtonID))
	if err := loginButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("LoginButton doesn't exist: ", err)
	}

	// Press until KEYCODE_DPAD_DOWN login button is focused.
	// Press KEYCODE_DPAD_RIGHT and KEYCODE_ENTER to click on login button.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if loginBtnFocused, err := loginButton.IsFocused(ctx); err != nil {
			return errors.New("login button not focused yet")
		} else if !loginBtnFocused {
			d.PressKeyCode(ctx, ui.KEYCODE_DPAD_DOWN, 0)
			return errors.New("login button not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("Failed to focus on login button: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_DPAD_RIGHT, 0); err != nil {
		s.Fatal("Failed to click on KEYCODE_DPAD_RIGHT button: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to click on KEYCODE_ENTER button: ", err)
	}

	// Click on enter email address.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("enterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	// Click on emailid text field until the emailid text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if emailIDFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
			return errors.New("email text field not focused yet")
		} else if !emailIDFocused {
			enterEmailAddress.Click(ctx)
			return errors.New("email text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus EmailId: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.Starz.emailid")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterPassword does not exist: ", err)
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
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.Starz.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Check for signIn Button.
	signInButton := d.Object(ui.ID(loginID))
	if err := signInButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("signInButton doesn't exists: ", err)
	}

	// Click on signIn Button until not now button exist.
	signInButton = d.Object(ui.ID(loginID))
	notNowButton := d.Object(ui.ID(notNowID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := notNowButton.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on dimiss button to save password.
	dimissButton := d.Object(ui.ID(dismissButtonID))
	if err := dimissButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("dimissButton doesn't exists: ", err)
	} else if err := dimissButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on dimissButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}

}

// signOutOfStarz verifies app is signed out.
func signOutOfStarz(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		menuID   = "com.bydeluxe.d3.android.program.starz:id/action_more"
		logoutID = "com.bydeluxe.d3.android.program.starz:id/settings_bttn"
	)

	// Click on menu icon.
	menuIcon := d.Object(ui.ID(menuID))
	if err := menuIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("menuIcon doesn't exist and skipped logout: ", err)
		return
	} else if err := menuIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on menuIcon: ", err)
	}

	// Click on log out of Starz.
	logOutOfStarz := d.Object(ui.ID(logoutID))
	if err := logOutOfStarz.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("logOutOfStarz doesn't exist: ", err)
	} else if err := logOutOfStarz.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfStarz: ", err)
	}
}
