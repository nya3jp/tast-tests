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

// clamshellLaunchForDiscord launches Discord in clamshell mode.
var clamshellLaunchForDiscord = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForDiscord},
}

// touchviewLaunchForDiscord launches Discord in tablet mode.
var touchviewLaunchForDiscord = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForDiscord},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Discord,
		Desc:         "Functional test for Discord that installs the app also verifies it is logged in and that the main page is open, checks Discord correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForDiscord,
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
				Tests:      touchviewLaunchForDiscord,
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
				Tests:      clamshellLaunchForDiscord,
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
				Tests:      touchviewLaunchForDiscord,
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
			"arcappcompat.Discord.emailid", "arcappcompat.Discord.password"},
	})
}

// Discord test uses library for opting into the playstore and installing app.
// Checks Discord correctly changes the window states in both clamshell and touchview mode.
func Discord(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.discord"
		appActivity = ".app.AppActivity$Main"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForDiscord verifies Discord is logged in and
// verify Discord reached main activity page of the app.
func launchAppForDiscord(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInText             = "Login"
		textEditClassName      = "android.widget.EditText"
		enterEmailText         = "Email"
		emailOrPhoneNumberText = "Email or Phone Number"
		enterPasswordText      = "Password"
		homeIconID             = "com.discord:id/tabs_host_bottom_nav_friends_item"
		notNowID               = "android:id/autofill_save_no"
		verifyCaptchaID        = "com.discord:id/auth_captcha_verify"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signInText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SignIn Button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}

	// Click on emailid text field until the emailid text field is focused.
	enterEmailAddress := d.Object(ui.ClassName(textEditClassName), ui.Text(enterEmailText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("enterEmailAddress doesn't exist: ", err)
		enterEmailAddress = d.Object(ui.ClassName(textEditClassName), ui.Text(emailOrPhoneNumberText))
		if err := enterEmailAddress.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
			s.Error("enterEmailAddress doesn't exist: ", err)
		}
	}
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

	emailID := s.RequiredVar("arcappcompat.Discord.emailid")
	if err := kb.Type(ctx, emailID); err != nil {
		s.Fatal("Failed to enter emailID: ", err)
	}
	s.Log("Entered EmailAddress")

	// Click on password text field until the password text field is focused.
	enterPassword := d.Object(ui.ClassName(textEditClassName), ui.Text(enterPasswordText))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterPassword doesn't exist: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
			return errors.New("password text field not focused yet")
		} else if !pwdFocused {
			enterPassword.Click(ctx)
			return errors.New("Password text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.Discord.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on sign in button.
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SignIn Button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}

	// Click on not now button.
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Check for captcha.
	verifyCaptcha := d.Object(ui.ID(verifyCaptchaID))
	if err := verifyCaptcha.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("verifyCaptcha doesn't exist: ", err)
		testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
		// Check for homePageVerifier.
		homePageVerifier := d.Object(ui.ID(homeIconID))
		if err := homePageVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
			s.Fatal("homePageVerifier doesn't exist: ", err)
		}
	} else {
		s.Log("Verify by reCaptcha exists")
	}
}
