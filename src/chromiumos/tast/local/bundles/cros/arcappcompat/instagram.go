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

// clamshellLaunchForInstagram launches Instagram in clamshell mode.
var clamshellLaunchForInstagram = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForInstagram},
}

// touchviewLaunchForInstagram launches Instagram in tablet mode.
var touchviewLaunchForInstagram = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForInstagram},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Instagram,
		Desc:         "Functional test for Instagram that installs the app also verifies it is logged in and that the main page is open, checks Instagram correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForInstagram,
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
				Tests:      touchviewLaunchForInstagram,
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
				Tests:      clamshellLaunchForInstagram,
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
				Tests:      touchviewLaunchForInstagram,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Instagram.username", "arcappcompat.Instagram.password"},
	})
}

// Instagram test uses library for opting into the playstore and installing app.
// Checks Instagram correctly changes the window states in both clamshell and touchview mode.
func Instagram(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.instagram.android"
		appActivity = ".activity.MainTabActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForInstagram verifies Instagram is logged in and
// verify Instagram reached main activity page of the app.
func launchAppForInstagram(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		captchaID           = "recaptcha-anchor"
		dismissButtonID     = "android:id/button2"
		enterEmailAddressID = "com.instagram.android:id/login_username"
		loginButtonID       = "com.instagram.android:id/log_in_button"
		loginID             = "com.instagram.android:id/button_text"
		notNowID            = "android:id/autofill_save_no"
		passwordID          = "com.instagram.android:id/password"
	)

	// Check for login button.
	loginButton := d.Object(ui.ID(loginButtonID))
	if err := loginButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("LoginButton doesn't exist: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
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

	username := s.RequiredVar("arcappcompat.Instagram.username")
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
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.Instagram.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Check for signInButton.
	signInButton := d.Object(ui.ID(loginID))
	if err := signInButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("signInButton does not exist")
	}
	// Click on signIn Button until not now button exist.
	signInButton = d.Object(ui.ID(loginID))
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

	// Click on dimiss button to save password.
	dimissButton := d.Object(ui.ID(dismissButtonID))
	if err := dimissButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("dimissButton doesn't exists: ", err)
	} else if err := dimissButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on dimissButton: ", err)
	}

	// Check for captcha.
	checkForCaptcha := d.Object(ui.ID(captchaID))
	if err := checkForCaptcha.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("CheckForCaptcha does not exist")
	} else {
		s.Log("CheckForCaptcha does exist")
		return
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
