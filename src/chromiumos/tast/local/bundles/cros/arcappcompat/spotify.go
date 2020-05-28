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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForSpotify = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSpotify},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: reOpenWindowForSpotifyAndSignOutOfApp},
}

// TouchviewTests are placed here.
var touchviewTestsForSpotify = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSpotify},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: reOpenWindowForSpotifyAndSignOutOfApp},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Spotify,
		Desc:         "Functional test for Spotify that installs the app also verifies it is logged in and that the main page is open, checks Spotify correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForSpotify,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForSpotify,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForSpotify,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForSpotify,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Spotify.email", "arcappcompat.Spotify.password"},
	})
}

// Spotify test uses library for opting into the playstore and installing app.
// Checks Spotify correctly changes the window states in both clamshell and touchview mode.
func Spotify(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.spotify.music"
		appActivity = ".MainActivity"
	)

	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForSpotify verifies Spotify is logged in and
// verify Spotify reached main activity page of the app.
func launchAppForSpotify(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		loginText         = "LOG IN"
		neverButtonID     = "com.google.android.gms:id/credential_save_reject"
		noThanksText      = "NO THANKS"
		homeIconClassName = "android.widget.TextView"
		homeIconText      = "Home"
	)

	// Click on login button.
	clickOnLoginButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(loginText))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("clickOnLoginButton doesn't exist: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
	}

	// Press tab twice to click on enter email.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Enter email address.
	spotifyEmailID := s.RequiredVar("arcappcompat.Spotify.email")
	if err := kb.Type(ctx, spotifyEmailID); err != nil {
		s.Fatal("Failed to enter enterEmail: ", err)
	}
	s.Log("Entered enterEmail")

	// Press tab to click on enter password.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
	}

	password := s.RequiredVar("arcappcompat.Spotify.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on Login button.
	loginButton := d.Object(ui.Text(loginText))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Login Button doesn't exist: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}

	// Click on never button.
	neverButton := d.Object(ui.ID(neverButtonID))
	if err := neverButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Never Button doesn't exist: ", err)
	} else if err := neverButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on neverButton: ", err)
	}

	// Click on no thanks button.
	clickOnNoThanksButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(noThanksText))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}
	// ReLaunch the app to skip save password dialog box.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}

	defer act.Close()

	// Stop the current running activity
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app: ", err)
	}

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ClassName(homeIconClassName), ui.Text(homeIconText))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("home icon doesn't exist: ", err)
	}

}

// reOpenWindowForSpotifyAndSignOutOfApp Test "close and relaunch the app", verifies app launch successfully without crash or ANR and signout of an app.
func reOpenWindowForSpotifyAndSignOutOfApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	// Launch the app.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	s.Log("Created new app activity")

	defer act.Close()

	s.Log("Stop the current activity of the app")
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app: ", err)
	}

	testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

	// ReLaunch the activity.
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start app: ", err)
	}
	s.Log("App relaunched successfully")

	signOutOfSpotify(ctx, s, tconn, a, d, appPkgName, appActivity)
}

// signOutOfSpotify verifies app is signed out.
func signOutOfSpotify(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		settingsIconClassName = "android.widget.ImageButton"
		settingsIconDes       = "Settings"
		scrollLayoutID        = "android:id/list"
		logoutID              = "android:id/text1"
		logOutOfSpotifyText   = "Log out"
	)

	// Click on settings icon.
	settingsIcon := d.Object(ui.ClassName(settingsIconClassName), ui.Description(settingsIconDes))
	if err := settingsIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SettingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}
	// Scroll until logout is visible.
	scrollLayout := d.Object(ui.ID(scrollLayoutID), ui.Scrollable(true))
	if err := scrollLayout.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LogOutOfSpotify doesn't exist: ", err)
	}

	logOutOfSpotify := d.Object(ui.ID(logoutID), ui.Text(logOutOfSpotifyText))
	scrollLayout.ScrollTo(ctx, logOutOfSpotify)
	if err := logOutOfSpotify.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LogOutOfSpotify doesn't exist: ", err)
	}
	// Click on log out of Spotify.
	logOutOfSpotify = d.Object(ui.ID(logoutID), ui.Text(logOutOfSpotifyText))
	if err := logOutOfSpotify.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogOutOfSpotify doesn't exist: ", err)
	} else if err := logOutOfSpotify.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfSpotify: ", err)
	}
}
