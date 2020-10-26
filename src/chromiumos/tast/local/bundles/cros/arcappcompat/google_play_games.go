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
var clamshellTestsForGooglePlayGames = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForGooglePlayGames},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForGooglePlayGames = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForGooglePlayGames},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GooglePlayGames,
		Desc:         "Functional test for GooglePlayGames that install, launch the app and check that the main page is open, also checks GooglePlayGames correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForGooglePlayGames,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForGooglePlayGames,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForGooglePlayGames,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForGooglePlayGames,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GooglePlayGames test uses library for opting into the playstore and installing app.
// Checks GooglePlayGames correctly changes the window states in both clamshell and touchview mode.
func GooglePlayGames(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.play.games"
		appActivity = "com.google.android.gms.games.ui.destination.main.MainActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForGooglePlayGames verifies GooglePlayGames is launched and
// verify GooglePlayGames reached main activity page of the app.
func launchAppForGooglePlayGames(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		homeID     = "com.google.android.play.games:id/games__navigation__bottom_navigation_view"
		createText = "Create"
		gotItText  = "Got it"
	)
	// Click on create button.
	clickOnCreateButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(createText))
	if err := clickOnCreateButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnCreateButton doesn't exist: ", err)
	} else if err := clickOnCreateButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnCreateButton: ", err)
	}

	// Click on gotit button.
	clickOnGotItButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(gotItText))
	if err := clickOnGotItButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnGotItButton doesn't exist: ", err)
	} else if err := clickOnGotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnGotItButton: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("homeIcon doesn't exist: ", err)
	}
}
