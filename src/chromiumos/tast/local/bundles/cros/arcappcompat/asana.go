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

// clamshellLaunchForAsana launches Asana in clamshell mode.
var clamshellLaunchForAsana = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForAsana},
}

// touchviewLaunchForAsana launches Asana in tablet mode.
var touchviewLaunchForAsana = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForAsana},
}

// clamshellAppSpecificTestsForAsana are placed here.
var clamshellAppSpecificTestsForAsana = []testutil.TestSuite{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForAsana are placed here.
var touchviewAppSpecificTestsForAsana = []testutil.TestSuite{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Asana,
		Desc:         "Functional test for Asana that installs the app also verifies it is logged in and that the main page is open, checks Asana correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForAsana,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForAsana,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForAsana,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForAsana,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForAsana,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForAsana,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForAsana,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForAsana,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Asana.username", "arcappcompat.Asana.password"},
	})
}

// Asana test uses library for opting into the playstore and installing app.
// Checks Asana correctly changes the window states in both clamshell and touchview mode.
func Asana(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.asana.app"
		appActivity = "com.asana.ui.activities.LaunchActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForAsana verifies Asana is logged in and
// verify Asana reached main activity page of the app.
func launchAppForAsana(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		logInText         = "Log In"
		emailID           = "com.asana.app:id/email"
		continueEmailText = "Continue with email"
		typePasswordText  = "Type password"
		passwordID        = "com.asana.app:id/password"
		nextButtonText    = "NEXT"
	)

	// Click on log in button
	logInButton := d.Object(ui.Text(logInText))
	if err := logInButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("LogIn button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on LogIn button: ", err)
	}

	// Enter email.
	AsanaEmailID := s.RequiredVar("arcappcompat.Asana.username")
	enterEmailAddress := d.Object(ui.ID(emailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.SetText(ctx, AsanaEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Click on continue button
	continueButton := d.Object(ui.Text(continueEmailText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("continue button doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continue button: ", err)
	}

	// Click on type password button.
	typePasswordButton := d.Object(ui.Text(typePasswordText))
	if err := typePasswordButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("typePassword button doesn't exists: ", err)
	} else if err := typePasswordButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on typePassword button: ", err)
	}

	// Enter password.
	AsanaPassword := s.RequiredVar("arcappcompat.Asana.password")
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterPassword doesn't exists: ", err)
	} else if err := enterPassword.SetText(ctx, AsanaPassword); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}

	// Click log in button.
	logInButton = d.Object(ui.Text(logInText))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogIn button doesn't exist: ", err)
	}

	// Click on loginbutton until next button exists.
	nextButton := d.Object(ui.TextMatches("(?i)" + nextButtonText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := nextButton.Exists(ctx); err != nil {
			logInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("nextButton doesn't exists: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
