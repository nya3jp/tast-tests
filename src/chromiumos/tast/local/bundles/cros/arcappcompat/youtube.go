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

// clamshellLaunchForYoutube launches Youtube in clamshell mode.
var clamshellLaunchForYoutube = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForYoutube},
}

// touchviewLaunchForYoutube launches Youtube in tablet mode.
var touchviewLaunchForYoutube = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForYoutube},
}

// clamshellAppSpecificTestsForYoutube are placed here.
var clamshellAppSpecificTestsForYoutube = []testutil.TestCase{
	// {Name: "Clamshell: Stylus click", Fn: testutil.StylusClick},
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForYoutube are placed here.
var touchviewAppSpecificTestsForYoutube = []testutil.TestCase{
	// {Name: "Touchview: Stylus click", Fn: testutil.StylusClick},
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Youtube,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Youtube that installs the app also verifies it is logged in and that the main page is open, checks Gmail correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForYoutube,
				CommonTests:      testutil.ClamshellSmokeTests,
				AppSpecificTests: clamshellAppSpecificTestsForYoutube,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForYoutube,
				CommonTests:      testutil.TouchviewSmokeTests,
				AppSpecificTests: touchviewAppSpecificTestsForYoutube,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForYoutube,
				CommonTests:      testutil.ClamshellSmokeTests,
				AppSpecificTests: clamshellAppSpecificTestsForYoutube,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForYoutube,
				CommonTests:      testutil.TouchviewSmokeTests,
				AppSpecificTests: touchviewAppSpecificTestsForYoutube,
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

// Youtube test uses library for opting into the playstore and installing app.
// Checks Youtube correctly changes the window states in both clamshell and touchview mode.
func Youtube(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.youtube"
		appActivity = "com.google.android.apps.youtube.app.WatchWhileActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForYoutube verifies app is logged in and
// verify app reached main activity page of the app.
func launchAppForYoutube(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
