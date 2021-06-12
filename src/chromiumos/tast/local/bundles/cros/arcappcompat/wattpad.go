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

// clamshellLaunchForWattpad launches Wattpad in clamshell mode.
var clamshellLaunchForWattpad = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForWattpad},
}

// touchviewLaunchForWattpad launches Wattpad in tablet mode.
var touchviewLaunchForWattpad = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForWattpad},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Wattpad,
		Desc:         "Functional test for Wattpad that installs the app also verifies it is logged in and that the main page is open, checks Wattpad correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForWattpad,
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
				Tests:      touchviewLaunchForWattpad,
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
				Tests:      clamshellLaunchForWattpad,
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
				Tests:      touchviewLaunchForWattpad,
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
			"arcappcompat.Wattpad.emailid", "arcappcompat.Wattpad.password"},
	})
}

// Wattpad test uses library for opting into the playstore and installing app.
// Checks Wattpad correctly changes the window states in both clamshell and touchview mode.
func Wattpad(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "wp.wattpad"
		appActivity = ".ui.activities.LoginActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForWattpad verifies Wattpad is logged in and
// verify Wattpad reached main activity page of the app.
func launchAppForWattpad(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInID          = "wp.wattpad:id/log_in_button"
		enterEmailText    = "Username"
		enterPasswordText = "Password"
		startText         = "Start Reading"
		notNowText        = "NOT NOW"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("signIn button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signIn button: ", err)
	}

	// Enter email address.
	WattpadEmailID := s.RequiredVar("arcappcompat.Wattpad.emailid")
	enterEmailAddress := d.Object(ui.Text(enterEmailText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, WattpadEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	WattpadPassword := s.RequiredVar("arcappcompat.Wattpad.password")
	enterPassword := d.Object(ui.Text(enterPasswordText))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, WattpadPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on start reading button.
	startButton := d.Object(ui.Text(startText))
	if err := startButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Start button doesn't exist: ", err)
	} else if err := startButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on start button: ", err)
	}

	// Check for not now button and press back to skip it.
	notNowButton := d.Object(ui.Text(notNowText))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		s.Log("Failed to press back button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
