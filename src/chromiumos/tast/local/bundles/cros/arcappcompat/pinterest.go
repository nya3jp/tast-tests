// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForPinterest = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForPinterest},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForPinterest = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForPinterest},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pinterest,
		Desc:         "Functional test for Pinterest that installs the app also verifies it is logged in and that the main page is open, checks Pinterest correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForPinterest,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForPinterest,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForPinterest,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForPinterest,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Pinterest test uses library for opting into the playstore and installing app.
// Checks Pinterest correctly changes the window states in both clamshell and touchview mode.
func Pinterest(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.pinterest"
		appActivity = ".activity.PinterestActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForPinterest verifies Pinterest is logged in and
// verify Pinterest reached main activity page of the app.
func launchAppForPinterest(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText                = "ALLOW"
		emailAddressID                 = "com.google.android.gms:id/account_name"
		loginWithGoogleButtonClassName = "android.widget.Button"
		loginWithGoogleButtonText      = "Continue with Google"
		profileIconID                  = "com.pinterest:id/profile_menu_view"
		turnOnLocationText             = "Turn on location services"
	)

	loginWithGoogleButton := d.Object(ui.ClassName(loginWithGoogleButtonClassName), ui.Text(loginWithGoogleButtonText))
	emailAddress := d.Object(ui.ID(emailAddressID))
	// Keep clicking login with Google Button until EmailAddress exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := emailAddress.Exists(ctx); err != nil {
			loginWithGoogleButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("emailAddress doesn't exist: ", err)
	}
	// Click on email address.
	if err := emailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	}

	// Click on turn on location button.
	turnOnLocationButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(turnOnLocationText))
	if err := turnOnLocationButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("turnOnLocationButton doesn't exist: ", err)
	} else if err := turnOnLocationButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on turnOnLocationButton: ", err)
	}

	// Keep clicking allow button until profile icon exists.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	profileIcon := d.Object(ui.ID(profileIconID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := profileIcon.Exists(ctx); err != nil {
			s.Log("Click on allow button")
			allowButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("profileIcon doesn't exists: ", err)
	}

	signOutOfPinterest(ctx, s, a, d, appPkgName, appActivity)

}

// signOutOfPinterest verifies app is signed out.
func signOutOfPinterest(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		accountIconID              = "com.pinterest:id/profile_menu_view"
		profileIconID              = "com.pinterest:id/user_profile_collapsed_avatar_container"
		settingsIconClassName      = "android.widget.ImageView"
		settingsIconDescription    = "Settings"
		logOutOfPinterestClassName = "android.widget.TextView"
		logOutOfPinterestText      = "Log out"
	)

	// Click on account icon.
	accountIcon := d.Object(ui.ID(accountIconID))
	if err := accountIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("AccountIcon doesn't exist: ", err)
	} else if err := accountIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on accountIcon: ", err)
	}

	// Click on profile icon.
	profileIcon := d.Object(ui.ID(profileIconID))
	if err := profileIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("ProfileIcon doesn't exist: ", err)
	} else if err := profileIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on ProfileIcon: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ClassName(settingsIconClassName), ui.Description(settingsIconDescription))
	if err := settingsIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SettingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}

	// Click on log out of Pinterest.
	logOutOfPinterest := d.Object(ui.ClassName(logOutOfPinterestClassName), ui.Text(logOutOfPinterestText))
	if err := logOutOfPinterest.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogOutOfPinterest doesn't exist: ", err)
	} else if err := logOutOfPinterest.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfPinterest: ", err)
	}
}
