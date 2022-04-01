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

// clamshellLaunchForHbomax launches Hbomax in clamshell mode.
var clamshellLaunchForHbomax = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForHbomax},
}

// touchviewLaunchForHbomax launches Hbomax in tablet mode.
var touchviewLaunchForHbomax = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForHbomax},
}

// clamshellAppSpecificTestsForHbomax are placed here.
var clamshellAppSpecificTestsForHbomax = []testutil.TestCase{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForHbomax are placed here.
var touchviewAppSpecificTestsForHbomax = []testutil.TestCase{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Hbomax,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Hbomax that install, launch the app and check that the main page is open, also checks Hbomax correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("eve")),
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForHbomax,
				CommonTests:      testutil.ClamshellSmokeTests,
				AppSpecificTests: clamshellAppSpecificTestsForHbomax,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForHbomax,
				CommonTests:      testutil.TouchviewSmokeTests,
				AppSpecificTests: touchviewAppSpecificTestsForHbomax,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForHbomax,
				CommonTests:      testutil.ClamshellSmokeTests,
				AppSpecificTests: clamshellAppSpecificTestsForHbomax,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForHbomax,
				CommonTests:      testutil.TouchviewSmokeTests,
				AppSpecificTests: touchviewAppSpecificTestsForHbomax,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 30 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
	})
}

// Hbomax test uses library for opting into the playstore and installing app.
// Checks Hbomax correctly changes the window states in both clamshell and touchview mode.
func Hbomax(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.hbo.hbonow"
		appActivity = "com.hbo.go.LaunchActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForHbomax verifies Hbomax is launched and
// verify Hbomax reached main activity page of the app.
func launchAppForHbomax(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
