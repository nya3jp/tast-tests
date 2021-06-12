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
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForMathKids launches MathKids in clamshell mode.
var clamshellLaunchForMathKids = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForMathKids},
}

// touchviewLaunchForMathKids launches MathKids in tablet mode.
var touchviewLaunchForMathKids = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForMathKids},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MathKids,
		Desc:         "Functional test for MathKids that installs the app also verifies that the main page is open, checks MathKids correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForMathKids,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForMathKids,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForMathKids,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForMathKids,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// MathKids test uses library for opting into the playstore and installing app.
// Checks MathKids correctly changes the window states in both clamshell and touchview mode.
func MathKids(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.rvappstudios.math.kids.counting"
		appActivity = "com.unity3d.player.UnityPlayerActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForMathKids verifies MathKids reached main activity page of the app.
func launchAppForMathKids(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		checkForPressAnyKeyPageClassName = "android.view.View"
		checkForPressKeyToContineText    = "Press any key to continue..."
		continueClassName                = "android.widget.FrameLayout"
		demoDes                          = "Game view"
		languageText                     = "Language"
		shortTimeInterval                = 300 * time.Millisecond
	)
	// Wait for app to load the page.
	d.WaitForIdle(ctx, testutil.ShortUITimeout)

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

	// Press enter key to click on continue button.
	clickOnContinueButton := d.Object(ui.ClassName(continueClassName), ui.PackageName(appPkgName))
	if err := clickOnContinueButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnContinueButton doesn't exists: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		d.WaitForIdle(ctx, shortTimeInterval)
		s.Fatal("Failed to press enter to click on continue button: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		d.WaitForIdle(ctx, shortTimeInterval)
		s.Fatal("Failed to press enter to click on continue button: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		d.WaitForIdle(ctx, shortTimeInterval)
		s.Fatal("Failed to press enter to click on continue button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
