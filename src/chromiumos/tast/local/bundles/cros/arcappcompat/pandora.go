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
var clamshellTestsForPandora = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForPandora},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForPandora = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForPandora},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pandora,
		Desc:         "Functional test for Pandora that installs the app also verifies it is logged in and that the main page is open, checks Pandora correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForPandora,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForPandora,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForPandora,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForPandora,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Pandora.emailid", "arcappcompat.Pandora.password"},
	})
}

// Pandora test uses library for opting into the playstore and installing app.
// Checks Pandora correctly changes the window states in both clamshell and touchview mode.
func Pandora(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.pandora.android"
		appActivity = ".LauncherActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForPandora verifies Pandora is logged in and
// verify Pandora reached main activity page of the app.
func launchAppForPandora(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInID         = "com.pandora.android:id/welcome_log_in_button"
		noneOfTheAboveID = "com.google.android.gms:id/cancel"
		enterEmailID     = "com.pandora.android:id/email_editText"
		enterPasswordID  = "com.pandora.android:id/password_editText"
		logInText        = "Log In"
		logInID          = "com.pandora.android:id/loading_text"
		profileIconID    = "com.pandora.android:id/tab_profile"
		profileIconDes   = "Profile"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Sign in button: ", err)
	}

	// Check and click email address.
	pandoraEmailID := s.RequiredVar("arcappcompat.Pandora.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	// Click on none of the above button.
	noneOfTheAboveButton := d.Object(ui.ID(noneOfTheAboveID))
	if err := noneOfTheAboveButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("noneOfTheAboveButton doesn't exist: ", err)
	} else if err := noneOfTheAboveButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on noneOfTheAboveButton: ", err)
	}

	//Set email address
	if err := enterEmailAddress.SetText(ctx, pandoraEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	pandoraPassword := s.RequiredVar("arcappcompat.Pandora.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, pandoraPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on log in button
	logInButton := d.Object(ui.ID(logInID), ui.Text(logInText))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogIn button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on LogIn button: ", err)
	}

	// Check for profile icon.
	profileIcon := d.Object(ui.ID(profileIconID), ui.Description(profileIconDes))
	if err := profileIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("profileIcon doesn't exist: ", err)
	}
}
