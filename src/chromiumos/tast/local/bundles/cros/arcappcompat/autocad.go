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

// clamshellLaunchForAutocad launches Autocad in clamshell mode.
var clamshellLaunchForAutocad = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForAutocad},
}

// touchviewLaunchForAutocad launches Autocad in tablet mode.
var touchviewLaunchForAutocad = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForAutocad},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Autocad,
		Desc:         "Functional test for Autocad that installs the app also verifies it is logged in and that the main page is open, checks Autocad correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForAutocad,
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
				Tests:      touchviewLaunchForAutocad,
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
				Tests:      clamshellLaunchForAutocad,
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
				Tests:      touchviewLaunchForAutocad,
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
			"arcappcompat.Autocad.emailid", "arcappcompat.Autocad.password"},
	})
}

// Autocad test uses library for opting into the playstore and installing app.
// Checks Autocad correctly changes the window states in both clamshell and touchview mode.
func Autocad(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.autodesk.autocadws"
		appActivity = ".view.activities.StartupActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForAutocad verifies Autocad is logged in and
// verify Autocad reached main activity page of the app.
func launchAppForAutocad(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		laterText        = "Later"
		signInText       = "Sign in"
		enterEmailID     = "userName"
		nextText         = "Next button"
		enterPasswordID  = "password"
		submitID         = "btnSubmit"
		notNowAndroidID  = "android:id/autofill_save_no"
		notNowAutodeskID = "com.autodesk.autocadws:id/notNowButton"
		okID             = "android:id/button1"
		notNowID         = "com.autodesk.autocadws:id/tpf_not_now"
		plusID           = "com.autodesk.autocadws:id/fabButton"
	)

	// Skip later dialog.
	laterButton := d.Object(ui.Text(laterText))
	if err := laterButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0)
		d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0)
	} else if err := laterButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on later button: ", err)
	}

	// Click on sign in button.
	signInButton := d.Object(ui.Text(signInText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign in button: ", err)
	}

	// Enter email address.
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	// Click on emailid text field until the emailid text field is focused.
	AutocadEmailID := s.RequiredVar("arcappcompat.Autocad.emailid")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if emailIDFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
			return errors.Wrap(err, "failed to call IsFocused")
		} else if !emailIDFocused {
			if err := enterEmailAddress.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click on enter email address")
			}
			return errors.New("email text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus email text field: ", err)
	} else if err := enterEmailAddress.SetText(ctx, AutocadEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	//Click next button
	nextButton := d.Object(ui.Text(nextText))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("next button doesn't exist: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to next button: ", err)
	}

	// Enter password.
	AutocadPassword := s.RequiredVar("arcappcompat.Autocad.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.SetText(ctx, AutocadPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on log in button
	logInButton := d.Object(ui.ID(submitID))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogIn button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on LogIn button: ", err)
	}

	// Click on not now android button.
	notNowAndroidButton := d.Object(ui.ID(notNowAndroidID))
	if err := notNowAndroidButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowAndroid button doesn't exist: ", err)
	} else if err := notNowAndroidButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowAndroid button: ", err)
	}

	// Click on not now autocad button.
	notNowAutocadButton := d.Object(ui.ID(notNowAutodeskID))
	if err := notNowAutocadButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowAutocad button doesn't exist: ", err)
	} else if err := notNowAutocadButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowAutocad button: ", err)
	}

	//Click ok button
	okButton := d.Object(ui.ID(okID))
	if err := okButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Ok button doesn't exist: ", err)
	} else if err := okButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Ok button: ", err)
	}

	//Click not now button
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Not now button doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on not now button: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for homePageVerifier.
	homePageVerifier := d.Object(ui.ID(plusID))
	if err := homePageVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("homePageVerifier doesn't exist: ", err)
	}
}
