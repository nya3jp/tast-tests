// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

// clamshellLaunchForInkredible launches  Inkredible in clamshell mode.
var clamshellLaunchForInkredible = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForInkredible},
}

// touchviewLaunchForInkredible launches  Inkredible in tablet mode.
var touchviewLaunchForInkredible = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForInkredible},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Inkredible,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Inkredible that installs the app also verifies it is logged in and that the main page is open, checks Inkredible correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForInkredible,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForInkredible,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForInkredible,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForInkredible,
				CommonTests: testutil.TouchviewCommonTests,
			},
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

// Inkredible test uses library for opting into the playstore and installing app.
// Checks Inkredible correctly changes the window states in both clamshell and touchview mode.
func Inkredible(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.viettran.INKredible"
		appActivity = ".ui.PPageMainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForInkredible verifies Inkredible is logged in and
// verify Inkredible reached main activity page of the app.
func launchAppForInkredible(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText      = "ALLOW"
		noThanksButtonText   = "No, thanks."
		toggleButtonID       = "android:id/switch_widget"
		imageButtonClassName = "android.widget.ImageButton"
		navigationDes        = "Back"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Enable on toggle button to allow access to manage file.
	enableToggleButton := d.Object(ui.ID(toggleButtonID))
	if err := enableToggleButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("enableToggleButton doesn't exist: ", err)
	} else if err := enableToggleButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on enableToggleButton: ", err)
	}

	// Click on navigation button to goto app screen.
	clickOnNavigationButton := d.Object(ui.ClassName(imageButtonClassName), ui.DescriptionMatches("(?i)"+navigationDes))
	if err := clickOnNavigationButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnNavigationButton doesn't exist: ", err)
	} else if err := clickOnNavigationButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNavigationButton: ", err)
	}

	// Click on noThanks Button.
	clickOnNoThanksButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+noThanksButtonText))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
