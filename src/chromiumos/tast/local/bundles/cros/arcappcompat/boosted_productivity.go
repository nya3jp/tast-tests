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

// clamshellLaunchForBoostedProductivity launches BoostedProductivity in clamshell mode.
var clamshellLaunchForBoostedProductivity = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForBoostedProductivity},
}

// touchviewLaunchForBoostedProductivity launches BoostedProductivity in tablet mode.
var touchviewLaunchForBoostedProductivity = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForBoostedProductivity},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BoostedProductivity,
		Desc:         "Functional test for BoostedProductivity that installs the app also verifies it is logged in and that the main page is open, checks BoostedProductivity correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForBoostedProductivity,
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
				Tests:      touchviewLaunchForBoostedProductivity,
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
				Tests:      clamshellLaunchForBoostedProductivity,
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
				Tests:      touchviewLaunchForBoostedProductivity,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.BoostedProductivity.emailid", "arcappcompat.BoostedProductivity.password"},
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// BoostedProductivity test uses library for opting into the playstore and installing app.
// Checks BoostedProductivity correctly changes the window states in both clamshell and touchview mode.
func BoostedProductivity(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.boostedproductivity.app"
		appActivity = ".activities.LauncherActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForBoostedProductivity verifies BoostedProductivity is logged in and
// verify BoostedProductivity reached main activity page of the app.
func launchAppForBoostedProductivity(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		acceptText = "Accept & continue"
	)

	// Click on accept and continue button.
	acceptButton := d.Object(ui.Text(acceptText))
	if err := acceptButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Accept button doesn't exist: ", err)
	} else if err := acceptButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on accept button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
