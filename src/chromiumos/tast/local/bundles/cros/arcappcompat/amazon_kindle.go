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
var clamshellTestsForAmazonKindle = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForAmazonKindle},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForAmazonKindle = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForAmazonKindle},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AmazonKindle,
		Desc:         "Functional test for AmazonKindle that installs the app also verifies it is logged in and that the main page is open, checks AmazonKindle correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForAmazonKindle,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForAmazonKindle,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForAmazonKindle,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForAmazonKindle,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.AmazonKindle.username", "arcappcompat.AmazonKindle.password"},
	})
}

// AmazonKindle test uses library for opting into the playstore and installing app.
// Checks AmazonKindle correctly changes the window states in both clamshell and touchview mode.
func AmazonKindle(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.amazon.kindle"
		appActivity = ".UpgradePage"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForAmazonKindle verifies AmazonKindle is logged in and
// verify AmazonKindle reached main activity page of the app.
func launchAppForAmazonKindle(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInAmazonButtonText = "Sign In with Amazon"
		textViewClassName      = "android.widget.TextView"
		myStuffDes             = "Home, Tab"
		enterEmailAddressID    = "ap_email"
		passwordClassName      = "android.widget.EditText"
		passwordID             = "ap_password"
		passwordText           = "Amazon password"
		signInText             = "Sign-In"
		sendOTPText            = "Send OTP"
		homeClassName          = "android.widget.LinearLayout"
		homeDes                = "More, Tab"
		notNowID               = "android:id/autofill_save_no"
		dismissButtonID        = "android:id/button2"
		importantMessageText   = "Important"
	)

	// Check for home icon.
	homeIcon := d.Object(ui.ClassName(homeClassName), ui.Description(homeDes))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("home icon doesn't exist: ", err)
	} else if err := homeIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on homeIcon: ", err)
	}
	// Click on signin with amazon button.
	signInWithAmazonButton := d.Object(ui.ClassName(textViewClassName), ui.Text(signInAmazonButtonText))
	if err := signInWithAmazonButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("signInWithAmazonButton doesn't exists: ", err)
	} else if err := signInWithAmazonButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInWithAmazonButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("EnterEmailAddress does not exist: ", err)
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

	username := s.RequiredVar("arcappcompat.AmazonKindle.username")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("EnterPassword does not exist: ", err)
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

	password := s.RequiredVar("arcappcompat.AmazonKindle.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on signIn Button until not now button exist.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signInText))
	sendOTPButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(sendOTPText))
	notNowButton := d.Object(ui.ID(notNowID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := notNowButton.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
		// Check for send OTP button
		if err := sendOTPButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Log("Send OTP Button doesn't exist: ", err)
		} else {
			s.Error("Failed to signed into the app")
		}
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on dimiss button to save password.
	dimissButton := d.Object(ui.ID(dismissButtonID))
	if err := dimissButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("dimissButton doesn't exists: ", err)
	} else if err := dimissButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on dimissButton: ", err)
	}

	// Check for captcha.
	checkForCaptcha := d.Object(ui.TextStartsWith(importantMessageText))
	if err := checkForCaptcha.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("checkForCaptcha doesn't exists: ", err)

		signoutOfAmazonKindle(ctx, s, a, d, appPkgName, appActivity)
	} else {
		s.Log("checkForCaptcha does exist")
	}

}

// signoutOfAmazonKindle verifies app is signed out.
func signoutOfAmazonKindle(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		linearClassName        = "android.widget.LinearLayout"
		homeDes                = "More, Tab"
		SignOutText            = "SIGN OUT"
		signInAmazonButtonText = "Sign In with Amazon"
		textViewClassName      = "android.widget.TextView"
	)
	var signOutIndex = 6

	// Click on signin with amazon button.
	signInWithAmazonButton := d.Object(ui.ClassName(textViewClassName), ui.Text(signInAmazonButtonText))
	if err := signInWithAmazonButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("signInWithAmazonButton doesn't exists: ", err)
	} else {
		s.Fatal("Failed to signed into the app")
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ClassName(linearClassName), ui.Description(homeDes))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("home icon doesn't exist: ", err)
	} else if err := homeIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on homeIcon: ", err)
	}
	// Select signout.
	selectSignoutOption := d.Object(ui.ClassName(linearClassName), ui.Index(signOutIndex))
	if err := selectSignoutOption.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("selectSignoutOption doesn't exist: ", err)
	} else if err := selectSignoutOption.Click(ctx); err != nil {
		s.Log("Failed to click on selectSignoutOption: ", err)
	}

	// Click on sign out button.
	signOutButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(SignOutText))
	if err := signOutButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SignOutButton doesn't exist: ", err)
	} else if err := signOutButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signOutButton: ", err)
	}
}
