// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

// clamshellLaunchForHomescapes launches Homescapes in clamshell mode.
var clamshellLaunchForHomescapes = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForHomescapes},
}

// touchviewLaunchForHomescapes launches Homescapes in tablet mode.
var touchviewLaunchForHomescapes = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForHomescapes},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Homescapes,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test for Homescapes that installs the app also verifies it is logged in and that the main page is open, checks Homescapes correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForHomescapes,
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
				LaunchTests: touchviewLaunchForHomescapes,
				CommonTests: testutil.TouchviewSmokeTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForHomescapes,
				CommonTests: testutil.ClamshellSmokeTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForHomescapes,
				CommonTests: testutil.TouchviewSmokeTests,
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

// Homescapes test uses library for opting into the playstore and installing app.
// Checks Homescapes correctly changes the window states in both clamshell and touchview mode.
func Homescapes(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.playrix.homescapes"
		appActivity = ".GoogleActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForHomescapes verify app is logged in and
// verify app reached main activity page of the app.
func launchAppForHomescapes(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		gmailAccountPageID = "com.google.android.gms:id/account_picker"
		okButtonText       = "OK"
	)

	// Click on cancel button to install google play games.
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Wait for Gmail account page.
	gmailAccountPage := d.Object(ui.ID(gmailAccountPageID))
	if err := gmailAccountPage.WaitForExists(ctx, testutil.LongUITimeout); err == nil {
		s.Log("gmailAccountPage does exists: ", err)
		// For selecting Gmail account
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		} else {
			s.Log("Entered KEYCODE_TAB")
		}
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	}

	// Click on ok button to agree with the app policy.
	okButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+okButtonText))
	if err := okButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("okButton doesn't exists: ", err)
	} else if err := okButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on okButton: ", err)
	}

	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
