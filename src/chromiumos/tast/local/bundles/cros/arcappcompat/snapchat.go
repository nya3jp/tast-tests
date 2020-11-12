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
var clamshellTestsForSnapchat = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSnapchat},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForSnapchat = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSnapchat},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Snapchat,
		Desc:         "Functional test for Snapchat that installs the app also verifies it is logged in and that the main page is open, checks Snapchat correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForSnapchat,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForSnapchat,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForSnapchat,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForSnapchat,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Snapchat.username", "arcappcompat.Snapchat.password"},
	})
}

// Snapchat test uses library for opting into the playstore and installing app.
// Checks Snapchat correctly changes the window states in both clamshell and touchview mode.
func Snapchat(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.snapchat.android"
		appActivity = "com.snapchat.android.LandingPageActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForSnapchat verifies Snapchat is logged in and
// verify Snapchat reached main activity page of the app.
func launchAppForSnapchat(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText             = "ALLOW"
		cameraID                    = "com.snapchat.android:id/ngs_camera_icon_container"
		enterEmailAddressID         = "com.snapchat.android:id/username_or_email_field"
		loginButtonClassName        = "android.widget.TextView"
		loginText                   = "Log In"
		signInID                    = "com.snapchat.android:id/nav_button"
		notNowID                    = "android:id/autofill_save_no"
		passwordID                  = "com.snapchat.android:id/password_field"
		profileID                   = "com.snapchat.android:id/neon_header_avatar_container"
		turnonText                  = "TURN ON"
		homeID                      = "com.bydeluxe.d3.android.program.Snapchat:id/action_home"
		whileUsingThisAppButtonText = "WHILE USING THE APP"
	)

	// Check for login button.
	loginButton := d.Object(ui.ClassName(loginButtonClassName), ui.TextMatches("(?i)"+loginText))
	if err := loginButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("LoginButton doesn't exist: ", err)
	}

	// click on login button until emailAddress exists.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := enterEmailAddress.Exists(ctx); err != nil {
			loginButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Error("enterEmailAddress button doesn't exists: ", err)
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

	username := s.RequiredVar("arcappcompat.Snapchat.username")
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

	password := s.RequiredVar("arcappcompat.Snapchat.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on signIn Button until not now button exist.
	signInButton := d.Object(ui.ID(signInID))
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

	// Click on turnon button to save password.
	turnonButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+turnonText))
	if err := turnonButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("turnonButton doesn't exists: ", err)
	} else if err := turnonButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on turnonButton: ", err)
	}

	// Click on allow button for accessing files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err = allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton Button: ", err)
	}

	if err = allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton Button: ", err)
	}

	// Click on allow while using this app button to record audio.
	clickOnWhileUsingThisAppButton := d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))
	if err = clickOnWhileUsingThisAppButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnWhileUsingThisApp Button doesn't exists: ", err)
	} else if err := clickOnWhileUsingThisAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnWhileUsingThisApp Button: ", err)
	}

	if err = allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton Button: ", err)
	}

	// Click on allow while using this app button to record video.
	clickOnWhileUsingThisAppButton = d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))
	if err = clickOnWhileUsingThisAppButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnWhileUsingThisApp Button doesn't exists: ", err)
	} else if err := clickOnWhileUsingThisAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnWhileUsingThisApp Button: ", err)
	}

	// Check for profile icon.
	profileIcon := d.Object(ui.ID(profileID))
	if err := profileIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("profileIcon doesn't exist: ", err)
	}
}
