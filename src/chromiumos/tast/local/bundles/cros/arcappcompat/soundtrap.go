// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForSoundtrap launches Soundtrap in clamshell mode.
var clamshellLaunchForSoundtrap = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForSoundtrap},
}

// touchviewLaunchForSoundtrap launches Soundtrap in tablet mode.
var touchviewLaunchForSoundtrap = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForSoundtrap},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Soundtrap,
		Desc:         "Functional test for Soundtrap that installs the app also verifies it is logged in and that the main page is open, checks Soundtrap correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForSoundtrap,
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
				Tests:      touchviewLaunchForSoundtrap,
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
				Tests:      clamshellLaunchForSoundtrap,
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
				Tests:      touchviewLaunchForSoundtrap,
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
			"arcappcompat.Soundtrap.emailid", "arcappcompat.Soundtrap.password"},
	})
}

// Soundtrap test uses library for opting into the playstore and installing app.
// Checks Soundtrap correctly changes the window states in both clamshell and touchview mode.
func Soundtrap(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.soundtrap.studioapp"
		appActivity = ".MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForSoundtrap verifies Soundtrap is logged in and
// verify Soundtrap reached main activity page of the app.
func launchAppForSoundtrap(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInText        = "Log in"
		enterEmailText    = "Email or Username"
		enterPasswordText = "Password"
		logInText         = "Log In"
		noThanksID        = "android:id/autofill_save_no"
		dismissText       = "Dismiss"
		nextText          = "Next"
		doneText          = "Done"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.Text(signInText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign in button: ", err)
	}

	// Check and click email address.
	SoundtrapEmailID := s.RequiredVar("arcappcompat.Soundtrap.emailid")
	enterEmailAddress := d.Object(ui.Text(enterEmailText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.SetText(ctx, SoundtrapEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	SoundtrapPassword := s.RequiredVar("arcappcompat.Soundtrap.password")
	enterPassword := d.Object(ui.Text(enterPasswordText))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.SetText(ctx, SoundtrapPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on log in button
	logInButton := d.Object(ui.Text(logInText))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogIn button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on LogIn button: ", err)
	}

	// Click on no thanks button.
	noThanksButton := d.Object(ui.ID(noThanksID))
	if err := noThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("noThanks button doesn't exist: ", err)
	} else if err := noThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on noThanks button: ", err)
	}

	// Click on dismiss button.
	dismissButton := d.Object(ui.Text(dismissText))
	if err := dismissButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("dismiss button doesn't exist: ", err)
	} else if err := dismissButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on dismiss button: ", err)
	}

	nextButton := d.Object(ui.Text(nextText))
	doneButton := d.Object(ui.Text(doneText))
	// Click on next button until done button exist in the home page.
	// Click on done button.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := doneButton.Exists(ctx); err != nil {
			nextButton.Click(ctx)
			return err
		} else if err := doneButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on done button: ", err)
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("next button doesn't exist: ", err)
	} else {
		s.Log("next button does exists")
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
