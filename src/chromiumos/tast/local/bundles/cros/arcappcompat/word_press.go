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
var clamshellTestsForWordPress = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForWordPress},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForWordPress = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForWordPress},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WordPress,
		Desc:         "Functional test for WordPress that installs the app also verifies it is logged in and that the main page is open, checks WordPress correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForWordPress,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForWordPress,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForWordPress,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForWordPress,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// WordPress test uses library for opting into the playstore and installing app.
// Checks WordPress correctly changes the window states in both clamshell and touchview mode.
func WordPress(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "org.wordpress.android"
		appActivity = ".ui.WPLaunchActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForWordPress verifies WordPress is logged in and
// verify WordPress reached main activity page of the app.
func launchAppForWordPress(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueWithWordPressID = "org.wordpress.android:id/first_button"
		continueWithGoogleID    = "org.wordpress.android:id/continue_with_google"
		notNowText              = "NOT RIGHT NOW"
		readerText              = "Reader"
	)

	// Click on continue with Wordpress button.
	continueButton := d.Object(ui.ID(continueWithWordPressID))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Continue button doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continue button: ", err)
	}

	// Click on google button.
	googleButton := d.Object(ui.ID(continueWithGoogleID))
	if err := googleButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Google button doesn't exist: ", err)
	} else if err := googleButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on google button: ", err)
	}

	// Click on not now button.
	notNowButton := d.Object(ui.Text(notNowText))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Not now button doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on not now button: ", err)
	}

	// Check for reader label.
	navReaderLabel := d.Object(ui.Text(readerText))
	if err := navReaderLabel.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("Reader label doesn't exist: ", err)
	}
}
