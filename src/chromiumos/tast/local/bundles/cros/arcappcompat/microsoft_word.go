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

// clamshellLaunchForMicrosoftWord launches MicrosoftWord in clamshell mode.
var clamshellLaunchForMicrosoftWord = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForMicrosoftWord},
}

// touchviewLaunchForMicrosoftWord launches MicrosoftWord in tablet mode.
var touchviewLaunchForMicrosoftWord = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForMicrosoftWord},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MicrosoftWord,
		Desc:         "Functional test for MicrosoftWord that installs the app also verifies it is logged in and that the main page is open, checks MicrosoftWord correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForMicrosoftWord,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForMicrosoftWord,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForMicrosoftWord,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForMicrosoftWord,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.MicrosoftWord.emailid", "arcappcompat.MicrosoftWord.password"},
	})
}

// MicrosoftWord test uses library for opting into the playstore and installing app.
// Checks MicrosoftWord correctly changes the window states in both clamshell and touchview mode.
func MicrosoftWord(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.microsoft.office.word"
		appActivity = "com.microsoft.office.apphost.LaunchActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForMicrosoftWord verifies MicrosoftWord is logged in and
// verify MicrosoftWord reached main activity page of the app.
func launchAppForMicrosoftWord(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText       = "ALLOW"
		enterEmailAddressID   = "com.microsoft.office.word:id/OfcEditText"
		nextButtonDescription = "Next"
		okText                = "OK"
		passwordClassName     = "android.widget.EditText"
		passwordID            = "i0118"
		passwordText          = "Password"
		signInClassName       = "android.widget.Button"
		signInText            = "Sign in"
		newID                 = "com.microsoft.office.word:id/docsui_landing_pane_header_heading"
		notNowID              = "android:id/autofill_save_no"
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

	emailAddress := s.RequiredVar("arcappcompat.MicrosoftWord.emailid")
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
			return errors.New("Password text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.MicrosoftWord.password")
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

	// Click on notnow button.
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
