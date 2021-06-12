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

// clamshellLaunchForTuneIn launches TuneIn in clamshell mode.
var clamshellLaunchForTuneIn = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForTuneIn},
}

// touchviewLaunchForTuneIn launches TuneIn in tablet mode.
var touchviewLaunchForTuneIn = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForTuneIn},
}

// clamshellAppSpecificTestsForTuneIn are placed here.
var clamshellAppSpecificTestsForTuneIn = []testutil.TestSuite{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForTuneIn are placed here.
var touchviewAppSpecificTestsForTuneIn = []testutil.TestSuite{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Tunein,
		Desc:         "Functional test for TuneIn that installs the app also verifies it is logged in and that the main page is open, checks TuneIn  correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForTuneIn,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForTuneIn,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForTuneIn,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForTuneIn,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForTuneIn,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForTuneIn,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForTuneIn,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForTuneIn,
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

// Tunein test uses library for opting into the playstore and installing app.
// Checks TuneIn correctly changes the window states in both clamshell and touchview mode.
func Tunein(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "tunein.player"
		appActivity = "tunein.ui.activities.upsell.UpsellWebViewActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForTuneIn verifies TuneIn is logged in and
// verify TuneIn reached main activity page of the app.
func launchAppForTuneIn(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		closeButtonText          = "Close Button"
		continueText             = "Continue with"
		allowText                = "ALLOW"
		noneOFTheAboveButtonText = "None Of The Above"
	)

	// Click on close button.
	closeButton := d.Object(ui.Text(closeButtonText))
	if err := closeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("closeButton doesn't exist: ", err)
	} else if err := closeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on closeButton: ", err)
	}

	// Continue with an account.
	ContinueButton := d.Object(ui.Text(continueText))
	if err := ContinueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Continue Button doesn't exist: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Fatal("Failed to press KEYCODE_TAB : ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to press ENTER key: ", err)
	}

	// Click on none of the above button.
	clickOnNoneOfTheAboveButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+noneOFTheAboveButtonText))
	if err := clickOnNoneOfTheAboveButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoneOfTheAboveButton doesn't exist: ", err)
	} else if err := clickOnNoneOfTheAboveButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoneOfTheAboveButton: ", err)
	}

	// Click on allow button.
	clickOnAllowButton := d.Object(ui.Text(allowText))
	if err := clickOnAllowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := clickOnAllowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAllowButton: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
