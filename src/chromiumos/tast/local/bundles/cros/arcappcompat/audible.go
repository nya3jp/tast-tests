// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

// clamshellLaunchForAudible launches Audible in clamshell mode.
var clamshellLaunchForAudible = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForAudible},
}

// touchviewLaunchForAudible launches Audible in tablet mode.
var touchviewLaunchForAudible = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForAudible},
}

// clamshellAppSpecificTestsForAudible are placed here.
var clamshellAppSpecificTestsForAudible = []testutil.TestCase{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForAudible are placed here.
var touchviewAppSpecificTestsForAudible = []testutil.TestCase{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Audible,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Audible that installs the app also verifies that the main page is open, checks Audible correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForAudible,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForAudible,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForAudible,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForAudible,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForAudible,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForAudible,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForAudible,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForAudible,
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

// Audible test uses library for opting into the playstore and installing app.
// Checks Audible correctly changes the window states in both clamshell and touchview mode.
func Audible(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.audible.application"
		appActivity = ".SplashScreen"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForAudible verifies Audible reached main activity page of the app.
func launchAppForAudible(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		getStartedButtonText = "Get started"
		homeDes              = "Search"
	)

	// Click on get started button.
	getStartedButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+getStartedButtonText))
	if err := getStartedButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("getStartedButton doesn't exist: ", err)
	} else if err := getStartedButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on getStartedButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for homePageVerifier.
	homePageVerifier := d.Object(ui.DescriptionMatches("(?i)" + homeDes))
	if err := homePageVerifier.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Fatal("homePageVerifier doesn't exists: ", err)
	}
}
