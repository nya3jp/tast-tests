// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForMessenger launches Messenger in clamshell mode.
var clamshellLaunchForMessenger = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForMessenger},
}

// touchviewLaunchForMessenger launches Messenger in tablet mode.
var touchviewLaunchForMessenger = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForMessenger},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Messenger,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Messenger that installs the app also verifies it is logged in and that the main page is open, checks Messenger correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForMessenger,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForMessenger,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForMessenger,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForMessenger,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.Messenger.emailid", "arcappcompat.Messenger.password"},
	})
}

// Messenger test uses library for opting into the playstore and installing app.
// Checks Messenger correctly changes the window states in both clamshell and touchview mode.
func Messenger(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.facebook.orca"
		appActivity = ".auth.StartScreenActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForMessenger verifies Messenger is logged in and
// verify Messenger reached main activity page of the app.
func launchAppForMessenger(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		cameraDes          = "Camera"
		doneDes            = "DONE"
		loginDes           = "LOG IN"
		notNowDes          = "NOT NOW"
		notNowText         = "NOT NOW"
		notNowID           = "android:id/autofill_save_no"
		neverButtonID      = "com.google.android.gms:id/credential_save_reject"
		okText             = "OK"
		passwordDes        = "Password"
		textClassName      = "android.widget.EditText"
		userNameDes        = "Phone Number or Email"
		viewGroupClassName = "android.view.ViewGroup"
	)

	// Enter email address.
	MessengerEmailID := s.RequiredVar("arcappcompat.Messenger.emailid")
	enterEmailAddress := d.Object(ui.ClassName(textClassName), ui.DescriptionMatches("(?i)"+userNameDes))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if enterEmailAddress.SetText(ctx, MessengerEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter Password.
	MessengerPassword := s.RequiredVar("arcappcompat.Messenger.password")
	enterPassword := d.Object(ui.ClassName(textClassName), ui.DescriptionMatches("(?i)"+passwordDes))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if enterPassword.SetText(ctx, MessengerPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	clickOnLoginButton := d.Object(ui.ClassName(viewGroupClassName), ui.DescriptionMatches("(?i)"+loginDes))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("clickOnLoginButton doesn't exist: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
	}

	// Click on never button.
	neverButton := d.Object(ui.ID(neverButtonID))
	if err := neverButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Never Button doesn't exist: ", err)
	} else if err := neverButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on neverButton: ", err)
	}

	// Click on not now button for saving password.
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on not now button to save info.
	clickOnNotNowButton := d.Object(ui.Text(notNowText))
	if err := clickOnNotNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNotNowButton doesn't exist: ", err)
	} else if err := clickOnNotNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNotNowButton: ", err)
	}

	// Click on not now button for adding contacts.
	clickOnNotNowButton = d.Object(ui.ClassName(viewGroupClassName), ui.Description(notNowDes))
	if err := clickOnNotNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNotNowButton doesn't exist: ", err)
	} else if err := clickOnNotNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNotNowButton: ", err)
	}
	// Click on ok button.
	clickOnOkButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(okText))
	if err := clickOnOkButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnOkButton doesn't exist: ", err)
	} else if err := clickOnOkButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnOkButton: ", err)
	}
	// Click on done button.
	clickOnDoneButton := d.Object(ui.ClassName(viewGroupClassName), ui.Description(doneDes))
	if err := clickOnDoneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnDoneButton doesn't exist: ", err)
	} else if err := clickOnDoneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnDoneButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
