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
var clamshellTestsForABCKids = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForABCKids},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForABCKids = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForABCKids},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ABCKids,
		Desc:         "Functional test for ABCKids that installs the app also verifies that the main page is open, checks ABCKids correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForABCKids,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForABCKids,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForABCKids,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForABCKids,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// ABCKids test uses library for opting into the playstore and installing app.
// Checks ABCKids correctly changes the window states in both clamshell and touchview mode.
func ABCKids(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.rvappstudios.abc_kids_toddler_tracing_phonics"
		appActivity = "com.unity3d.player.UnityPlayerActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForABCKids verifies ABCKids reached main activity page of the app.
func launchAppForABCKids(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		checkForPressAnyKeyPageClassName = "android.view.View"
		checkForPressKeyToContineText    = "Press any key to continue..."
		continueText                     = "Continue"
		demoDes                          = "Game view"
		shortTimeInterval                = 300 * time.Millisecond
	)

	// Check for press any key and press enter key to continue navigating the app.
	checkForPressAnyKeyPage := d.Object(ui.ClassName(checkForPressAnyKeyPageClassName), ui.PackageName(appPkgName))
	if err := checkForPressAnyKeyPage.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("checkForPressAnyKeyPage doesn't exists: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to press ENTER key: ", err)
	}

	// Press KEYCODE_DPAD_RIGHT and ENTER to close the demo video.
	checkForDemoPage := d.Object(ui.Description(demoDes))
	if err := checkForDemoPage.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("checkForDemoPage doesn't exists: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_DPAD_RIGHT, 0); err != nil {
		s.Fatal("Failed to press KEYCODE_DPAD_RIGHT : ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to press ENTER key: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
