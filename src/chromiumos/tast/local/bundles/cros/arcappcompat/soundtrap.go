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
)

// ClamshellTests are placed here.
var clamshellTestsForSoundtrap = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSoundtrap},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForSoundtrap = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSoundtrap},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Soundtrap,
		Desc:         "Functional test for Soundtrap that installs the app also verifies it is logged in and that the main page is open, checks Soundtrap correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForSoundtrap,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForSoundtrap,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForSoundtrap,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForSoundtrap,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
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
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
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
		homeIconText      = "Create New Project"
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

	// Check for home icon.
	homeIconButton := d.Object(ui.Text(homeIconText))
	if err := homeIconButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("homeIcon button doesn't exist: ", err)
	}
}
