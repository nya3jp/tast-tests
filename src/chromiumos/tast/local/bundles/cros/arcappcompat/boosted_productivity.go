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
var clamshellTestsForBoostedProductivity = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForBoostedProductivity},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForBoostedProductivity = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForBoostedProductivity},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BoostedProductivity,
		Desc:         "Functional test for BoostedProductivity that installs the app also verifies it is logged in and that the main page is open, checks BoostedProductivity correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForBoostedProductivity,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForBoostedProductivity,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForBoostedProductivity,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForBoostedProductivity,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.BoostedProductivity.emailid", "arcappcompat.BoostedProductivity.password"},
	})
}

// BoostedProductivity test uses library for opting into the playstore and installing app.
// Checks BoostedProductivity correctly changes the window states in both clamshell and touchview mode.
func BoostedProductivity(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.boostedproductivity.app"
		appActivity = ".activities.LauncherActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForBoostedProductivity verifies BoostedProductivity is logged in and
// verify BoostedProductivity reached main activity page of the app.
func launchAppForBoostedProductivity(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		acceptText = "Accept & continue"
		drawerID   = "com.boostedproductivity.app:id/iv_drawer_button"
	)

	// Click on accept and continue button.
	acceptButton := d.Object(ui.Text(acceptText))
	if err := acceptButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Accept button doesn't exist: ", err)
	} else if err := acceptButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on accept button: ", err)
	}

	// Check for drawer button.
	drawerButton := d.Object(ui.ID(drawerID))
	if err := drawerButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("Drawer button doesn't exist: ", err)
	}
}
