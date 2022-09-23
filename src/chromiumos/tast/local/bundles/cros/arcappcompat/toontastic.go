// Copyright 2022 The ChromiumOS Authors
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

// clamshellLaunchForToontastic launches Toontastic in clamshell mode.
var clamshellLaunchForToontastic = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForToontastic, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForToontastic launches Toontastic in tablet mode.
var touchviewLaunchForToontastic = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForToontastic, Timeout: testutil.LaunchTestCaseTimeout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Toontastic,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Toontastic that installs the app also verifies if the main page is open, checks Toontastic correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForToontastic,
				CommonTests: testutil.ClamshellCommonTests,
			},
			// Commented it since Toontastic is covered only for "appcompat_top_apps" suite.
			// ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForToontastic,
				CommonTests: testutil.TouchviewCommonTests,
			},
			// Commented it since Toontastic is covered only for "appcompat_top_apps" suite.
			// ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForToontastic,
				CommonTests: testutil.ClamshellCommonTests,
			},
			// Commented it since Toontastic is covered only for "appcompat_top_apps" suite.
			// ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForToontastic,
				CommonTests: testutil.TouchviewCommonTests,
			},
			// Commented it since Toontastic is covered only for "appcompat_top_apps" suite.
			// ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "clamshell_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForToontastic,
				TopAppTests: testutil.ClamshellTopAppTests,
			},
			ExtraAttr:         []string{"appcompat_top_apps"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForToontastic,
				TopAppTests: testutil.TouchviewTopAppTests,
			},
			ExtraAttr:         []string{"appcompat_top_apps"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForToontastic,
				TopAppTests: testutil.ClamshellTopAppTests,
			},
			ExtraAttr:         []string{"appcompat_top_apps"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForToontastic,
				TopAppTests: testutil.TouchviewTopAppTests,
			},
			ExtraAttr:         []string{"appcompat_top_apps"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 20 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
	})
}

// Toontastic test uses library for opting into the playstore and installing app.
// Checks Toontastic correctly changes the window states in both clamshell and touchview mode.
func Toontastic(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.toontastic"
		appActivity = "com.google.android.libraries.kids.creative.adapter.CreativeUnityAdapter"
	)

	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForToontastic verifies Toontastic is logged in and
// verify Toontastic reached main activity page of the app.
func launchAppForToontastic(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		continueText = "CONTINUE"
	)
	// Click on continue button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueText))
	if err := continueButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("continue button doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continue button: ", err)
	}

	// Click on allow button.
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
