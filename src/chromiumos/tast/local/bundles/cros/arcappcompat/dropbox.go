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
var clamshellTestsForDropbox = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForDropbox},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForDropbox = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForDropbox},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dropbox,
		Desc:         "Functional test for Dropbox that installs the app also verifies it is logged in and that the main page is open, checks Dropbox correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForDropbox,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForDropbox,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForDropbox,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForDropbox,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Dropbox.emailid", "arcappcompat.Dropbox.password"},
	})
}

// Dropbox test uses library for opting into the playstore and installing app.
// Checks Dropbox correctly changes the window states in both clamshell and touchview mode.
func Dropbox(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.dropbox.android"
		appActivity = ".activity.DbxMainActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForDropbox verifies Dropbox is logged in and
// verify Dropbox reached main activity page of the app.
func launchAppForDropbox(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInID          = "com.dropbox.android:id/tour_sign_in"
		enterEmailID      = "com.dropbox.android:id/login_email_text_view"
		enterPasswordID   = "com.dropbox.android:id/login_password_text_view"
		submitID          = "com.dropbox.android:id/login_submit"
		cancelDescription = "Cancel"
		skipText          = "Skip"
		homeFabID         = "com.dropbox.android:id/fab_button"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign in button: ", err)
	}

	// Check and click email address.
	DropboxEmailID := s.RequiredVar("arcappcompat.Dropbox.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, DropboxEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	DropboxPassword := s.RequiredVar("arcappcompat.Dropbox.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterPassword doesn't exist: ", err)
	} else if err := enterPassword.SetText(ctx, DropboxPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on submit button.
	submitButton := d.Object(ui.ID(submitID))
	if err := submitButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("submit button doesn't exist: ", err)
	} else if err := submitButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on submit  button: ", err)
	}

	// Click on cancel button.
	cancelButton := d.Object(ui.Description(cancelDescription))
	if err := cancelButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("cancel button doesn't exist: ", err)
	} else if err := cancelButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on cancel  button: ", err)
	}

	// Click on skip button.
	skipButton := d.Object(ui.Text(skipText))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("skip button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip  button: ", err)
	}

	// Check for fav icon.
	homeFavButton := d.Object(ui.ID(homeFabID))
	if err := homeFavButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("homeFavButton button doesn't exist: ", err)
	}
}
