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
var clamshellTestsForMicrosoftPowerpoint = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForMicrosoftPowerpoint},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForMicrosoftPowerpoint = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForMicrosoftPowerpoint},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MicrosoftPowerpoint,
		Desc:         "Functional test for MicrosoftPowerpoint that installs the app also verifies it is logged in and that the main page is open, checks MicrosoftPowerpoint correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForMicrosoftPowerpoint,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForMicrosoftPowerpoint,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForMicrosoftPowerpoint,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForMicrosoftPowerpoint,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.MicrosoftPowerpoint.emailid", "arcappcompat.MicrosoftPowerpoint.password"},
	})
}

// MicrosoftPowerpoint test uses library for opting into the playstore and installing app.
// Checks MicrosoftPowerpoint correctly changes the window states in both clamshell and touchview mode.
func MicrosoftPowerpoint(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.microsoft.office.powerpoint"
		appActivity = "com.microsoft.office.apphost.LaunchActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForMicrosoftPowerpoint verifies MicrosoftPowerpoint is logged in and
// verify MicrosoftPowerpoint reached main activity page of the app.
func launchAppForMicrosoftPowerpoint(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText       = "ALLOW"
		enterEmailAddressID   = "com.microsoft.office.powerpoint:id/OfcEditText"
		nextButtonDescription = "Next"
		okText                = "OK"
		notNowText            = "NOT NOW"
		passwordClassName     = "android.widget.EditText"
		passwordID            = "i0118"
		passwordText          = "Password"
		signInClassName       = "android.widget.Button"
		signInText            = "Sign in"
		newID                 = "com.microsoft.office.powerpoint:id/docsui_landing_pane_header_heading"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exists: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	emailAddress := s.RequiredVar("arcappcompat.MicrosoftPowerpoint.emailid")
	if err := enterEmailAddress.SetText(ctx, emailAddress); err != nil {
		s.Fatal("Failed to enter EmailAddress: ", err)
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
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	kbp, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kbp.Close()

	password := s.RequiredVar("arcappcompat.MicrosoftPowerpoint.password")
	if err := kbp.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on Sign in button.
	signInButton := d.Object(ui.ClassName(signInClassName), ui.Text(signInText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SignInButton doesn't exists: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}

	// Click on allow button to access your files.
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on ok button.
	okButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(okText))
	if err := okButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("okButton doesn't exists: ", err)
	} else if err := okButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on okButton: ", err)
	}

	// Click on not now button.
	notNowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(notNowText))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Check for newIcon on homePage.
	newIcon := d.Object(ui.ID(newID))
	if err := newIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("NewIcon doesn't exists: ", err)
	}
}
