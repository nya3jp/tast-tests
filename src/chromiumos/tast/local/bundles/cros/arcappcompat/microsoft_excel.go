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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// ClamshellTests are placed here.
var clamshellTestsForMicrosoftExcel = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForMicrosoftExcel},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForMicrosoftExcel = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForMicrosoftExcel},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MicrosoftExcel,
		Desc:         "Functional test for MicrosoftExcel that installs the app also verifies it is logged in and that the main page is open, checks MicrosoftExcel correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForMicrosoftExcel,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForMicrosoftExcel,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("careena"), hwdep.SkipOnModel("kasumi"), hwdep.SkipOnModel("treeya"),
				hwdep.SkipOnModel("bluebird"), hwdep.SkipOnModel("sarien"), hwdep.SkipOnModel("elemi"), hwdep.SkipOnModel("berknip"),
				hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("aleena"), hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("grunt"),
				hwdep.SkipOnModel("liara"), hwdep.SkipOnModel("nuwani"), hwdep.SkipOnModel("kindred"), hwdep.SkipOnModel("dratini"),
				hwdep.SkipOnModel("apel"), hwdep.SkipOnModel("blooglet"), hwdep.SkipOnModel("blorb"), hwdep.SkipOnModel("bobba"),
				hwdep.SkipOnModel("casta"), hwdep.SkipOnModel("dorp"), hwdep.SkipOnModel("droid"), hwdep.SkipOnModel("fleex"),
				hwdep.SkipOnModel("foob"), hwdep.SkipOnModel("garfour"), hwdep.SkipOnModel("garg"), hwdep.SkipOnModel("laser14"),
				hwdep.SkipOnModel("lick"), hwdep.SkipOnModel("mimrock"), hwdep.SkipOnModel("nospike"), hwdep.SkipOnModel("orbatrix"),
				hwdep.SkipOnModel("phaser"), hwdep.SkipOnModel("sparky")),
			Pre: pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForMicrosoftExcel,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForMicrosoftExcel,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("careena"), hwdep.SkipOnModel("kasumi"), hwdep.SkipOnModel("treeya"),
				hwdep.SkipOnModel("bluebird"), hwdep.SkipOnModel("sarien"), hwdep.SkipOnModel("elemi"), hwdep.SkipOnModel("berknip"),
				hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("aleena"), hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("grunt"),
				hwdep.SkipOnModel("liara"), hwdep.SkipOnModel("nuwani"), hwdep.SkipOnModel("kindred"), hwdep.SkipOnModel("dratini"),
				hwdep.SkipOnModel("apel"), hwdep.SkipOnModel("blooglet"), hwdep.SkipOnModel("blorb"), hwdep.SkipOnModel("bobba"),
				hwdep.SkipOnModel("casta"), hwdep.SkipOnModel("dorp"), hwdep.SkipOnModel("droid"), hwdep.SkipOnModel("fleex"),
				hwdep.SkipOnModel("foob"), hwdep.SkipOnModel("garfour"), hwdep.SkipOnModel("garg"), hwdep.SkipOnModel("laser14"),
				hwdep.SkipOnModel("lick"), hwdep.SkipOnModel("mimrock"), hwdep.SkipOnModel("nospike"), hwdep.SkipOnModel("orbatrix"),
				hwdep.SkipOnModel("phaser"), hwdep.SkipOnModel("sparky")),
			Pre: pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.MicrosoftExcel.emailid", "arcappcompat.MicrosoftExcel.password"},
	})
}

// MicrosoftExcel test uses library for opting into the playstore and installing app.
// Checks MicrosoftExcel correctly changes the window states in both clamshell and touchview mode.
func MicrosoftExcel(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.microsoft.office.excel"
		appActivity = "com.microsoft.office.apphost.LaunchActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForMicrosoftExcel verifies MicrosoftExcel is logged in and
// verify MicrosoftExcel reached main activity page of the app.
func launchAppForMicrosoftExcel(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText       = "ALLOW"
		enterEmailAddressID   = "com.microsoft.office.excel:id/OfcEditText"
		nextButtonDescription = "Next"
		okText                = "OK"
		notNowText            = "NOT NOW"
		passwordClassName     = "android.widget.EditText"
		passwordID            = "i0118"
		passwordText          = "Password"
		signInClassName       = "android.widget.Button"
		signInText            = "Sign in"
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

	// Click on enterEmailAddress until the email text field is focused.
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

	emailAddress := s.RequiredVar("arcappcompat.MicrosoftExcel.emailid")
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

	password := s.RequiredVar("arcappcompat.MicrosoftExcel.password")
	if err := enterPassword.SetText(ctx, password); err != nil {
		s.Fatal("Failed to enter enterPassword: ", err)
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

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
