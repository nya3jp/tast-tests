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

// clamshellLaunchForSpreakerStudio launches SpreakerStudio in clamshell mode.
var clamshellLaunchForSpreakerStudio = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForSpreakerStudio},
}

// touchviewLaunchForSpreakerStudio launches SpreakerStudio in tablet mode.
var touchviewLaunchForSpreakerStudio = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForSpreakerStudio},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SpreakerStudio,
		Desc:         "Functional test for SpreakerStudio that installs the app also verifies it is logged in and that the main page is open, checks SpreakerStudio correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForSpreakerStudio,
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
				Tests:      touchviewLaunchForSpreakerStudio,
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
				Tests:      clamshellLaunchForSpreakerStudio,
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
				Tests:      touchviewLaunchForSpreakerStudio,
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
			"arcappcompat.SpreakerStudio.emailid", "arcappcompat.SpreakerStudio.password"},
	})
}

// SpreakerStudio test uses library for opting into the playstore and installing app.
// Checks SpreakerStudio correctly changes the window states in both clamshell and touchview mode.
func SpreakerStudio(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.spreaker.android.studio"
		appActivity = ".MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForSpreakerStudio verifies SpreakerStudio is logged in and
// verify SpreakerStudio reached main activity page of the app.
func launchAppForSpreakerStudio(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText             = "ALLOW"
		enterEmailAddressID         = "com.spreaker.android.studio:id/login_spreaker_signin_email"
		loginText                   = "LOG IN / SIGN UP"
		notNowID                    = "android:id/autofill_save_no"
		iHaveAnAccountButtonText    = "Already have an account? SIGN IN"
		passwordID                  = "com.spreaker.android.studio:id/login_spreaker_signin_password"
		signInText                  = "SIGN IN"
		maybeLaterButtonText        = "MAYBE LATER"
		whileUsingThisAppButtonText = "WHILE USING THE APP"
	)

	// Click on login button.
	loginButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+loginText))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("loginButton doesn't exists: ", err)
	} else {
		s.Log("loginButton does exists: ", err)
		if err := loginButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on loginButton: ", err)
		}

		// Click on I have an account button.
		clickOnIHavaAnAccountButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+iHaveAnAccountButtonText))
		if err := clickOnIHavaAnAccountButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
			s.Log("clickOnIHavaAnAccountButton doesn't exists: ", err)
		} else if err := clickOnIHavaAnAccountButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on clickOnIHavaAnAccountButton: ", err)
		}

		// Enter email id.
		enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
		if err := enterEmailAddress.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
			s.Error("EnterEmailAddress doesn't exists: ", err)
		}

		// Click on email address text field until the email address text field is focused.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if emailAddressFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
				return errors.New("emailAddress text field not focused yet")
			} else if !emailAddressFocused {
				enterEmailAddress.Click(ctx)
				return errors.New("emailAddress text field not focused yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
			s.Fatal("Failed to focus emailAddress: ", err)
		}

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		defer kb.Close()

		emailAddress := s.RequiredVar("arcappcompat.SpreakerStudio.emailid")
		if err := kb.Type(ctx, emailAddress); err != nil {
			s.Fatal("Failed to enter emailAddress: ", err)
		}
		s.Log("Entered EmailAddress")

		// Enter password.
		enterPassword := d.Object(ui.ID(passwordID))
		if err := enterPassword.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
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

		password := s.RequiredVar("arcappcompat.SpreakerStudio.password")
		if err := kb.Type(ctx, password); err != nil {
			s.Fatal("Failed to enter password: ", err)
		}
		s.Log("Entered password")

		// Click on Sign in button.
		signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signInText))
		if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Error("SignInButton doesn't exists: ", err)
		} else if err := signInButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on signInButton: ", err)
		}

		// Click on notNow button.
		notNowButton := d.Object(ui.ID(notNowID))
		if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Log("notNowButton doesn't exist: ", err)
		} else if err := notNowButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on notNowButton: ", err)
		}
	}

	// Click on allow button.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on allow while using this app button to record audio.
	clickOnWhileUsingThisAppButton := d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))
	if err := clickOnWhileUsingThisAppButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnWhileUsingThisApp Button doesn't exists: ", err)
	} else if err := clickOnWhileUsingThisAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnWhileUsingThisApp Button: ", err)
	}

	// Click on maybe later button.
	maybelaterButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+maybeLaterButtonText))
	if err := maybelaterButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("maybelaterButton doesn't exists: ", err)
	} else if err := maybelaterButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on maybelaterButton: ", err)

	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
