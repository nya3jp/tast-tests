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

// clamshellLaunchForKahoot launches Kahoot in clamshell mode.
var clamshellLaunchForKahoot = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForKahoot},
}

// touchviewLaunchForKahoot launches Kahoot in tablet mode.
var touchviewLaunchForKahoot = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForKahoot},
}

// clamshellAppSpecificTestsForKahoot are placed here.
var clamshellAppSpecificTestsForKahoot = []testutil.TestSuite{
	{Name: "Clamshell: Signout app", Fn: signOutOfKahoot},
}

// touchviewAppSpecificTestsForKahoot are placed here.
var touchviewAppSpecificTestsForKahoot = []testutil.TestSuite{
	{Name: "Touchview: Signout app", Fn: signOutOfKahoot},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Kahoot,
		Desc:         "Functional test for Kahoot that installs the app also verifies it is logged in and that the main page is open, checks Kahoot correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForKahoot,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForKahoot,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForKahoot,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForKahoot,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForKahoot,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForKahoot,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForKahoot,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForKahoot,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Kahoot.emailid", "arcappcompat.Kahoot.password"},
	})
}

// Kahoot test uses library for opting into the playstore and installing app.
// Checks Kahoot correctly changes the window states in both clamshell and touchview mode.
func Kahoot(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "no.mobitroll.kahoot.android"
		appActivity = ".application.SplashActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForKahoot verifies Kahoot is logged in and
// verify Kahoot reached main activity page of the app.
func launchAppForKahoot(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueText        = "Continue with Basic"
		dismissButtonID     = "android:id/button2"
		enterEmailAddressID = "username"
		loginButtonID       = "no.mobitroll.kahoot.android:id/loginText"
		loginText           = "Log in"
		mayBeLaterText      = "Maybe later"
		notNowID            = "android:id/autofill_save_no"
		passwordID          = "password"
	)

	// Check for login button.
	loginButton := d.Object(ui.ID(loginButtonID))
	if err := loginButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("LoginButton doesn't exist: ", err)
	}

	// Press until KEYCODE_TAB login button is focused.
	// Press KEYCODE_DPAD_RIGHT and KEYCODE_ENTER to click on login button.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if loginBtnFocused, err := loginButton.IsFocused(ctx); err != nil {
			return errors.New("login button not focused yet")
		} else if !loginBtnFocused {
			d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0)
			return errors.New("login button not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("Failed to focus login button: ", err)
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

	username := s.RequiredVar("arcappcompat.Kahoot.emailid")
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

	password := s.RequiredVar("arcappcompat.Kahoot.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on signIn Button until not now button exist.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+loginText))
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

	// Click on maybe later button.
	maybeLaterButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+mayBeLaterText))
	if err := maybeLaterButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("maybeLaterButton doesn't exists: ", err)
	} else if err := maybeLaterButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on maybeLaterButton: ", err)
	}

	// Click on continue with basic button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueText))
	if err := continueButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
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

// signOutOfKahoot verifies app is signed out.
func signOutOfKahoot(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		profileID       = "no.mobitroll.kahoot.android:id/profileView"
		settingsID      = "no.mobitroll.kahoot.android:id/settings"
		logoutClassName = "android.widget.TextView"
		logoutText      = "Sign out"
		homeID          = "no.mobitroll.kahoot.android:id/homeTab"
	)
	// Check for homeIcon.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("homeIcon doesn't exist and skipped logout: ", err)
		return
	}

	// Click on profile icon.
	profileIcon := d.Object(ui.ID(profileID))
	if err := profileIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("profileIcon doesn't exist: ", err)
	} else if err := profileIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on profileIcon: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ID(settingsID))
	if err := settingsIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("settingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}

	// Click on log out of Kahoot.
	logOutOfKahoot := d.Object(ui.ClassName(logoutClassName), ui.TextMatches("(?i)"+logoutText))
	if err := logOutOfKahoot.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("logOutOfKahoot doesn't exist: ", err)
	} else if err := logOutOfKahoot.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfKahoot: ", err)
	}
}
