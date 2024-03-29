// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForColorAndShapes launches ColorAndShapes in clamshell mode.
var clamshellLaunchForColorAndShapes = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForColorAndShapes},
}

// touchviewLaunchForColorAndShapes launches ColorAndShapes in tablet mode.
var touchviewLaunchForColorAndShapes = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForColorAndShapes},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ColorAndShapes,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for ColorAndShapes that installs the app also verifies that the main page is open, checks ColorAndShapes correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForColorAndShapes,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForColorAndShapes,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForColorAndShapes,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForColorAndShapes,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
	})
}

// ColorAndShapes test uses library for opting into the playstore and installing app.
// Checks ColorAndShapes correctly changes the window states in both clamshell and touchview mode.
func ColorAndShapes(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.rvappstudios.shapes.colors.toddler"
		appActivity = "com.unity3d.player.UnityPlayerActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForColorAndShapes verifies ColorAndShapes reached main activity page of the app.
func launchAppForColorAndShapes(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
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
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
