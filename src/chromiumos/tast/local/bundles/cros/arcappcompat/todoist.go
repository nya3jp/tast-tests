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
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
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
		continueWithGoogleText = "CONTINUE WITH GOOGLE"
		emailAddressID         = "com.google.android.gms:id/container"
		termsID                = "com.todoist:id/terms_button"
		accountID              = "com.google.android.gms:id/account_display_name"
		fabID                  = "com.todoist:id/fab"
	)

	// Click on continue button.
	googleButton := d.Object(ui.Text(continueWithGoogleText))
	if err := googleButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Google button doesn't exist: ", err)
	} else if err := googleButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on google button: ", err)
	}

	// Click on email address.
	emailAddress := d.Object(ui.ID(emailAddressID))
	if err := emailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	}

	// Click on agree to terms Todoist button.
	termsButton := d.Object(ui.ID(termsID))
	if err := termsButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Terms button doesn't exist: ", err)
	} else if err := termsButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on terms button: ", err)
	}

	// Click on account.
	accountButton := d.Object(ui.ID(accountID))
	if err := accountButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Account button doesn't exist: ", err)
	} else if err := accountButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on account button: ", err)
	}

	// Press back button until home page exists.
	homeButton := d.Object(ui.ID(fabID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := homeButton.Exists(ctx); err != nil {
			if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
				s.Log("Failed to enter KEYCODE_BACK: ", err)
			} else {
				s.Log("Entered KEYCODE_BACK")
			}
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Home icon doesn't exist: ", err)
	}
}
