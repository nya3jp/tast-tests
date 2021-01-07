// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
var clamshellTestsForTodoist = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForTodoist},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForTodoist = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForTodoist},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Todoist,
		Desc:         "Functional test for Todoist that installs the app also verifies it is logged in and that the main page is open, checks Todoist correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForTodoist,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForTodoist,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForTodoist,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForTodoist,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Todoist.emailid", "arcappcompat.Todoist.password"},
	})
}

// Todoist test uses library for opting into the playstore and installing app.
// Checks Todoist correctly changes the window states in both clamshell and touchview mode.
func Todoist(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.todoist"
		appActivity = ".activity.HomeActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForTodoist verifies Todoist is logged in and
// verify Todoist reached main activity page of the app.
func launchAppForTodoist(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueWithEmailText = "CONTINUE WITH EMAIL"
		noneID                = "com.google.android.gms:id/cancel"
		emailAddressID        = "com.todoist:id/email_exists_input"
		passwordID            = "com.todoist:id/log_in_password"
		logInText             = "LOG IN"
		fabID                 = "com.todoist:id/fab"
	)

	// Click on continue button.
	continueButton := d.Object(ui.Text(continueWithEmailText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Continue button doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continue button: ", err)
	}

	// Click on none button.
	noneButton := d.Object(ui.ID(noneID))
	if err := noneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("None button doesn't exist: ", err)
	} else if err := noneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on none button: ", err)
	}

	// Click on email address.
	TodoistEmailID := s.RequiredVar("arcappcompat.Todoist.emailid")
	emailAddress := d.Object(ui.ID(emailAddressID))
	if err := emailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	} else if err := emailAddress.SetText(ctx, TodoistEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Click on continue button.
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Continue button doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continue button: ", err)
	}

	// Enter password.
	TodoistPassword := s.RequiredVar("arcappcompat.Todoist.password")
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, TodoistPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on login button.
	logInButton := d.Object(ui.Text(logInText))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Log in button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on log in button: ", err)
	}

	// Check for home Icon.
	homeButton := d.Object(ui.ID(fabID))
	if err := homeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Fatal("homeButton doesn't exist: ", err)
	}
}
