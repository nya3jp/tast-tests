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
	"chromiumos/tast/testing/hwdep"
)

// ClamshellTests are placed here.
var clamshellTestsForCrossDJ = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForCrossDJ},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForCrossDJ = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForCrossDJ},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrossDJ,
		Desc:         "Functional test for CrossDJ that installs the app also verifies it is logged in and that the main page is open, checks CrossDJ correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("elm")),
		Params: []testing.Param{{
			Val:               clamshellTestsForCrossDJ,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBootedForCrossDJ,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForCrossDJ,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForCrossDJ,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForCrossDJ,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBootedForCrossDJ,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForCrossDJ,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForCrossDJ,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.CrossDJ.username", "arcappcompat.CrossDJ.password"},
	})
}

// CrossDJ test uses library for opting into the playstore and installing app.
// Checks  CrossDJ correctly changes the window states in both clamshell and touchview mode.
func CrossDJ(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.mixvibes.crossdjapp"
		appActivity = "com.mixvibes.crossdj.CrossDJActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForCrossDJ verifies app is logged in and
// verify app reached main activity page of the app.
func launchAppForCrossDJ(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		allowText = "ALLOW"
		recID     = "com.mixvibes.crossdjapp:id/recordMixerButton"
	)
	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.Text(allowText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log(" allow button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allow button: ", err)
	}

	// Check home page is launched.
	homeButton := d.Object(ui.ID(recID))
	if err := homeButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("home button doesn't exists: ", err)
	}
}
