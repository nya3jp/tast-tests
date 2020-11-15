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
)

// ClamshellTests are placed here.
var clamshellTestsForFlipboard = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForFlipboard},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForFlipboard = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForFlipboard},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Flipboard,
		Desc:         "Functional test for Flipboard that installs the app also verifies it is logged in and that the main page is open, checks Instagram correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForFlipboard,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForFlipboard,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForFlipboard,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForFlipboard,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Flipboard.username", "arcappcompat.Flipboard.password"},
	})
}

// Flipboard test uses library for opting into the playstore and installing app.
// Checks Flipboard correctly changes the window states in both clamshell and touchview mode.
func Flipboard(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "flipboard.app"
		appActivity = "flipboard.activities.LaunchActivityAlias"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForFlipboard verifies Flipboard is logged in and
// verify Flipboard reached main activity page of the app.
func launchAppForFlipboard(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInButtonID           = "flipboard.app:id/first_launch_cover_sign_in"
		EmailButtonID            = "flipboard.app:id/account_login_email_button"
		editTextClassName        = "android.widget.EditText"
		emailText                = "Email"
		passwordText             = "Password"
		nextID                   = "flipboard.app:id/account_login_email_next_button"
		flipID                   = "flipboard.app:id/cover_flip_hint"
		homeID                   = "flipboard.app:id/toc_page_avatar"
		noneOFTheAboveButtonText = "None Of The Above"
		notNowID                 = "android:id/autofill_save_no"
	)
	// Click on sign in button.
	clickOnSignInButton := d.Object(ui.ID(signInButtonID))
	if err := clickOnSignInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnSignInButton doesn't exist: ", err)
	} else if err := clickOnSignInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnSignInButton: ", err)
	}

	// Click on email button.
	clickOnEmailButton := d.Object(ui.ID(EmailButtonID))
	if err := clickOnEmailButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnEmailButton doesn't exist: ", err)
	} else if err := clickOnEmailButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnEmailButton: ", err)
	}

	// Click on none of the above button.
	clickOnNoneOfTheAboveButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+noneOFTheAboveButtonText))
	if err := clickOnNoneOfTheAboveButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoneOfTheAboveButton doesn't exist: ", err)
	} else if err := clickOnNoneOfTheAboveButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoneOfTheAboveButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ClassName(editTextClassName), ui.Text(emailText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterEmailAddress does not exist: ", err)
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
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus EmailId: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.Flipboard.username")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Click on next button.
	clickOnNextButton := d.Object(ui.ID(nextID))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}

	// Enter password.
	enterPassword := d.Object(ui.ClassName(editTextClassName), ui.Text(passwordText))
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
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.Flipboard.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on signin Button until flip button exist.
	signInButton := d.Object(ui.ID(nextID))
	notNowButton := d.Object(ui.ID(notNowID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := notNowButton.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Check for flip button.
	checkForflipButton := d.Object(ui.ID(flipID))
	if err := checkForflipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("checkForflipButton doesn't exist: ", err)
	}

	// Press KEYCODE_DPAD_RIGHT to flip the page to goto home page.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_DPAD_RIGHT, 0); err != nil {
		s.Log("Failed to enter KEYCODE_DPAD_RIGHT: ", err)
	} else {
		s.Log("Entered KEYCODE_DPAD_RIGHT")
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("homeIcon doesn't exist: ", err)
	} else {
		s.Log("homeIcon does exist")
		signOutOfFlipboard(ctx, s, a, d, appPkgName, appActivity)
	}

}

// signOutOfFlipboard verifies app is signed out.
func signOutOfFlipboard(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		accountIconID     = "flipboard.app:id/toc_page_avatar"
		settingsIconID    = "flipboard.app:id/profile_page_header_settings"
		selectSignOutID   = "android:id/title"
		selectSignOutText = "Sign Out"
	)

	// Click on account icon.
	accountIcon := d.Object(ui.ID(accountIconID))
	if err := accountIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("AccountIcon doesn't exist: ", err)
	} else if err := accountIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on accountIcon: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ID(settingsIconID))
	if err := settingsIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("settingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}

	// Click on select sign out.
	clickOnSelectSignOut := d.Object(ui.ID(selectSignOutID), ui.TextMatches("(?i)"+selectSignOutText))
	if err := clickOnSelectSignOut.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("clickOnSelectSignOut doesn't exist: ", err)
	} else if err := clickOnSelectSignOut.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnSelectSignOut: ", err)
	}

	// Click on log out of Flipboard.
	logOutOfFlipboard := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+selectSignOutText))
	if err := logOutOfFlipboard.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("logOutOfFlipboard doesn't exist: ", err)
	} else if err := logOutOfFlipboard.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfFlipboard: ", err)
	}
}
