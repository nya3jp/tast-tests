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
var clamshellTestsForMessenger = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForMessenger},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForMessenger = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForMessenger},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Messenger,
		Desc:         "Functional test for Messenger that installs the app also verifies it is logged in and that the main page is open, checks Messenger correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForMessenger,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForMessenger,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForMessenger,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForMessenger,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Messenger.emailid", "arcappcompat.Messenger.password"},
	})
}

// Messenger test uses library for opting into the playstore and installing app.
// Checks Messenger correctly changes the window states in both clamshell and touchview mode.
func Messenger(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.facebook.orca"
		appActivity = ".auth.StartScreenActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
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
		okText             = "OK"
		passwordDes        = "Password"
		textClassName      = "android.widget.EditText"
		userNameDes        = "Phone Number or Email"
		viewGroupClassName = "android.view.ViewGroup"
	)

	// Enter email address.
	MessengerEmailID := s.RequiredVar("arcappcompat.Messenger.emailid")
	enterEmailAddress := d.Object(ui.ClassName(textClassName), ui.Description(userNameDes))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if enterEmailAddress.SetText(ctx, MessengerEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter Password.
	MessengerPassword := s.RequiredVar("arcappcompat.Messenger.password")
	enterPassword := d.Object(ui.ClassName(textClassName), ui.Description(passwordDes))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if enterPassword.SetText(ctx, MessengerPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	clickOnLoginButton := d.Object(ui.ClassName(viewGroupClassName), ui.Description(loginDes))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnLoginButton doesn't exist: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
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

	// Check for camera Icon.
	cameraIcon := d.Object(ui.ClassName(viewGroupClassName), ui.Description(cameraDes))
	if err := cameraIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("cameraIcon doesn't exist: ", err)
	}
}
