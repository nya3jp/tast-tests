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
var clamshellTestsForMicrosoftOnedrive = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForMicrosoftOnedrive},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForMicrosoftOnedrive = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForMicrosoftOnedrive},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MicrosoftOnedrive,
		Desc:         "Functional test for MicrosoftOnedrive that installs the app also verifies it is logged in and that the main page is open, checks MicrosoftOnedrive correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForMicrosoftOnedrive,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForMicrosoftOnedrive,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForMicrosoftOnedrive,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForMicrosoftOnedrive,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.MicrosoftOnedrive.emailid", "arcappcompat.MicrosoftOnedrive.password"},
	})
}

// MicrosoftOnedrive test uses library for opting into the playstore and installing app.
// Checks MicrosoftOnedrive correctly changes the window states in both clamshell and touchview mode.
func MicrosoftOnedrive(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.microsoft.skydrive"
		appActivity = ".MainActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForMicrosoftOnedrive verifies MicrosoftOnedrive is logged in and
// verify MicrosoftOnedrive reached main activity page of the app.
func launchAppForMicrosoftOnedrive(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText        = "ALLOW"
		gotItButtonText        = "GOT IT"
		homeClassName          = "android.widget.TextView"
		hometext               = "Meet your Personal Vault"
		enterEmailAddressID    = "com.microsoft.skydrive:id/authentication_input_text"
		nextButtonDescription  = "Next"
		okText                 = "OK"
		notNowID               = "android:id/autofill_save_no"
		notnowText             = "NOT NOW"
		passwordClassName      = "android.widget.EditText"
		passwordID             = "i0118"
		passwordText           = "Password"
		signInClassName        = "android.widget.Button"
		signinText             = "SIGN IN"
		turnOnCameraUploadText = "TURN ON CAMERA UPLOAD"
	)

	// Click on signin button.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signinText))
	if err := signInButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("signInButton doesn't exists: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exists: ", err)
	}

	// Keep clicking enterEmailAddress until the email text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if enterEmailAddressFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
			return errors.New("enterEmailAddress not focused yet")
		} else if !enterEmailAddressFocused {
			enterEmailAddress.Click(ctx)
			return errors.New("enterEmailAddress not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus enterEmailAddress: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	emailAddress := s.RequiredVar("arcappcompat.MicrosoftOnedrive.emailid")
	if err := kb.Type(ctx, emailAddress); err != nil {
		s.Fatal("Failed to enter emailAddress: ", err)
	}
	s.Log("Entered EmailAddress")

	// Click on next button
	nextButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Description(nextButtonDescription))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Next Button doesn't exists: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterPassword doesn't exists: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	// Keep clicking password text field until the password text field is focused.
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

	password := s.RequiredVar("arcappcompat.MicrosoftOnedrive.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on Sign in button.
	signInButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signinText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SignInButton doesn't exists: ", err)
	}

	// Click on signin Button until flip button exist.
	signInButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signinText))
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

	// click on got it button.
	gotItButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+gotItButtonText))
	if err := gotItButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("gotItButton doesn't exists: ", err)
	} else if err := gotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on gotItButton: ", err)
	}

	// Click on turnOn Camera upload button.
	turnOnCameraUploadButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+turnOnCameraUploadText))
	if err := turnOnCameraUploadButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("turnOnCameraUploadButton doesn't exists: ", err)
	} else if err := turnOnCameraUploadButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on turnOnCameraUploadButton: ", err)
	}

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on notnow button for feedback.
	notnowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+notnowText))
	if err := notnowButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("notnowButton doesn't exists: ", err)
	} else if err := notnowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notnowButton: ", err)
	}

	// Check for homeIcon on homePage.
	homeIcon := d.Object(ui.ClassName(homeClassName), ui.Text(hometext))
	if err := homeIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Fatal("homeIcon doesn't exists: ", err)
	}
}
