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
var clamshellTestsForIHeartRadio = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForIHeartRadio},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForIHeartRadio = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForIHeartRadio},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         IHeartRadio,
		Desc:         "Functional test for IHeartRadio that installs the app also verifies it is logged in and that the main page is open, checks IHeartRadio correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForIHeartRadio,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForIHeartRadio,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForIHeartRadio,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForIHeartRadio,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.IHeartRadio.emailid", "arcappcompat.IHeartRadio.password"},
	})
}

// IHeartRadio test uses library for opting into the playstore and installing app.
// Checks IHeartRadio correctly changes the window states in both clamshell and touchview mode.
func IHeartRadio(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.clearchannel.iheartradio.controller"
		appActivity = ".activities.NavDrawerActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForIHeartRadio verifies IHeartRadio is logged in and
// verify IHeartRadio reached main activity page of the app.
func launchAppForIHeartRadio(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInID           = "com.clearchannel.iheartradio.controller:id/login_button"
		enterEmailID       = "com.clearchannel.iheartradio.controller:id/email"
		enterPasswordID    = "com.clearchannel.iheartradio.controller:id/password"
		logInID            = "com.clearchannel.iheartradio.controller:id/email_login"
		notNowText         = "NOT NOW"
		skipText           = "Skip"
		libraryDescription = "Your Library"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign in button: ", err)
	}

	// Check and click email address.
	IHeartRadioEmailID := s.RequiredVar("arcappcompat.IHeartRadio.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, IHeartRadioEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	IHeartRadioPassword := s.RequiredVar("arcappcompat.IHeartRadio.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterPassword doesn't exist: ", err)
	} else if err := enterPassword.SetText(ctx, IHeartRadioPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on login button.
	logIntButton := d.Object(ui.ID(logInID))
	if err := logIntButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("login  button doesn't exist: ", err)
	} else if err := logIntButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on login  button: ", err)
	}

	// Click on not now button.
	notNowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(notNowText))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on skip button.
	skipButton := d.Object(ui.Text(skipText))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("skip button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip button: ", err)
	}

	// Check for library button.
	libraryButton := d.Object(ui.Description(libraryDescription))
	if err := libraryButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("Library button doesn't exist: ", err)
	}
}
