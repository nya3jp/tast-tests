// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForAmazonPrimeVideo = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForAmazonPrimeVideo},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForAmazonPrimeVideo = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForAmazonPrimeVideo},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AmazonPrimeVideo,
		Desc:         "Functional test for AmazonPrimeVideo that installs the app also verifies it is logged in and that the main page is open, checks AmazonPrimeVideo correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForAmazonPrimeVideo,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForAmazonPrimeVideo,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForAmazonPrimeVideo,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForAmazonPrimeVideo,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.AmazonPrimeVideo.username", "arcappcompat.AmazonPrimeVideo.password"},
	})
}

// AmazonPrimeVideo test uses library for opting into the playstore and installing app.
// Checks AmazonPrimeVideo correctly changes the window states in both clamshell and touchview mode.
func AmazonPrimeVideo(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.amazon.avod.thirdpartyclient"
		appActivity = ".LauncherActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForAmazonPrimeVideo verifies AmazonPrimeVideo is logged in and
// verify AmazonPrimeVideo reached main activity page of the app.
func launchAppForAmazonPrimeVideo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText      = "ALLOW"
		textViewClassName    = "android.widget.TextView"
		myStuffText          = "My Stuff"
		enterEmailAddressID  = "ap_email"
		passwordClassName    = "android.widget.EditText"
		passwordID           = "ap_password"
		passwordText         = "Amazon password"
		signInText           = "Sign-In"
		sendOTPText          = "Send OTP"
		notNowID             = "android:id/autofill_save_no"
		importantMessageText = "Important"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterEmailAddress does not exist: ", err)
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

	username := s.RequiredVar("arcappcompat.AmazonPrimeVideo.username")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterPassword does not exist: ", err)
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

	password := s.RequiredVar("arcappcompat.AmazonPrimeVideo.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signInText))
	notNowButton := d.Object(ui.ID(notNowID))

	// Click on signIn Button until notNow Button exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := notNowButton.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log(" notNowButtondoesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Check for captcha and OTP.
	myStuffIcon := d.Object(ui.ClassName(textViewClassName), ui.Text(myStuffText))
	checkForCaptcha := d.Object(ui.TextStartsWith(importantMessageText))
	sendOTPButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(sendOTPText))

	if err := checkForCaptcha.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("checkForCaptcha doesn't exists: ", err)
		if err := sendOTPButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Log("Send OTP Button doesn't exist")
			//Check for myStuffIcon.
			if err := myStuffIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
				s.Error("MyStuffIcon doesn't exist: ", err)
			} else {
				signOutAmazonPrimeVideo(ctx, s, a, d, appPkgName, appActivity)
			}
		} else {
			s.Log("Send OTP Button does exist")
		}
	} else {
		s.Log("Check for captcha exist")
	}
}

// signOutAmazonPrimeVideo verifies app is signed out.
func signOutAmazonPrimeVideo(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		textViewClassName             = "android.widget.TextView"
		myStuffText                   = "My Stuff"
		settingsIconClassName         = "android.widget.ImageButton"
		settingsIconDescription       = "Settings"
		selectSignedInOptionClassName = "android.widget.TextView"
		selectSignedInOptionText      = "Signed in as Music Tester"
		signOutText                   = "SIGN OUT"
	)

	// Click on my stuff icon.
	myStuffIcon := d.Object(ui.ClassName(textViewClassName), ui.Text(myStuffText))
	if err := myStuffIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("MyStuffIcon doesn't exist: ", err)
	} else if err := myStuffIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click MyStuffIcon: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ClassName(settingsIconClassName), ui.DescriptionContains(settingsIconDescription))
	if err := settingsIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SettingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}
	// Select signed in option as music tester.
	selectSignedInOption := d.Object(ui.ClassName(selectSignedInOptionClassName), ui.Text(selectSignedInOptionText))
	if err := selectSignedInOption.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SelectSignedInOption doesn't exist: ", err)
	} else if err := selectSignedInOption.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectSignedInOption: ", err)
	}
	// Click on sign out button.
	signOutButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signOutText))
	if err := signOutButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SignOutButton doesn't exist: ", err)
	} else if err := signOutButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signOutButton: ", err)
	}
}
