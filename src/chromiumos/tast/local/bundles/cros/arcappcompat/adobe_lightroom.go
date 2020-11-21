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
var clamshellTestsForAdobeLightroom = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForAdobeLightroom},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForAdobeLightroom = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForAdobeLightroom},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdobeLightroom,
		Desc:         "Functional test for AdobeLightroom that installs the app also verifies it is logged in and that the main page is open, checks AdobeLightroom correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForAdobeLightroom,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForAdobeLightroom,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForAdobeLightroom,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForAdobeLightroom,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.AdobeLightroom.emailid", "arcappcompat.AdobeLightroom.password"},
	})
}

// AdobeLightroom test uses library for opting into the playstore and installing app.
// Checks AdobeLightroom correctly changes the window states in both clamshell and touchview mode.
func AdobeLightroom(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.adobe.lrmobile"
		appActivity = ".lrimport.ptpimport.PtpActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForAdobeLightroom verifies AdobeLightroom is logged in and
// verify AdobeLightroom reached main activity page of the app.
func launchAppForAdobeLightroom(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		skipID          = "com.adobe.lrmobile:id/gotoLastPage"
		signInID        = "com.adobe.lrmobile:id/signIn"
		enterEmailID    = "EmailPage-EmailField"
		continueText    = "Continue"
		enterPasswordID = "PasswordPage-PasswordField"
		addIconID       = "com.adobe.lrmobile:id/addPhotosButton"
	)

	// Click on skip button.
	skipButton := d.Object(ui.ID(skipID))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("skip button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip in button: ", err)
	}

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign in button: ", err)
	}

	// Check and click email address.
	AdobeLightroomEmailID := s.RequiredVar("arcappcompat.AdobeLightroom.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, AdobeLightroomEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Click on continue button.
	continueButton := d.Object(ui.Text(continueText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("continue button doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continue  button: ", err)
	}

	// Enter password.
	AdobeLightroomPassword := s.RequiredVar("arcappcompat.AdobeLightroom.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterPassword doesn't exist: ", err)
	} else if err := enterPassword.SetText(ctx, AdobeLightroomPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on continue button.
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("continue button doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continue  button: ", err)
	}

	// Check for add icon.
	addIconButton := d.Object(ui.ID(addIconID))
	if err := addIconButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("addIcon button doesn't exist: ", err)
	}
}
