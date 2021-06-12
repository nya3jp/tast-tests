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

// clamshellLaunchForNoteshelf launches Noteshelf in clamshell mode.
var clamshellLaunchForNoteshelf = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForNoteshelf},
}

// touchviewLaunchForNoteshelf launches Noteshelf in tablet mode.
var touchviewLaunchForNoteshelf = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForNoteshelf},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Noteshelf,
		Desc:         "Functional test for Noteshelf that install, launch the app and check that the main page is open, also checks Noteshelf correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForNoteshelf,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedForNoteshelf,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForNoteshelf,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeForNoteshelf,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForNoteshelf,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedForNoteshelf,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForNoteshelf,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeForNoteshelf,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.Noteshelf.username", "arcappcompat.Noteshelf.password"},
	})
}

// Noteshelf test uses library for opting into the playstore and installing app.
// Checks Noteshelf correctly changes the window states in both clamshell and touchview mode.
func Noteshelf(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.fluidtouch.noteshelf2"
		appActivity = "com.fluidtouch.noteshelf.commons.ui.FTSplashScreenActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForNoteshelf verifies Noteshelf is launched and
// verify Noteshelf reached main activity page of the app.
func launchAppForNoteshelf(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		agreeID           = "com.fluidtouch.noteshelf2:id/agreeCheckLayout"
		continueButtonID  = "com.fluidtouch.noteshelf2:id/welcome_screen1_continue_button"
		skipText          = "SKIP"
		startNoteTakingID = "com.fluidtouch.noteshelf2:id/welcome_screen5_start_button"
	)
	// Click on continue button.
	clickOnContinueButton := d.Object(ui.ID(continueButtonID))
	if err := clickOnContinueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnContinueButton doesn't exist: ", err)
	} else if err := clickOnContinueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnContinueButton: ", err)
	}

	// Click on skip button.
	clickOnSkipButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(skipText))
	if err := clickOnSkipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnSkipButton doesn't exist: ", err)
	} else if err := clickOnSkipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnSkipButton: ", err)
	}

	// Click on agree button.
	clickOnAgreeButton := d.Object(ui.ID(agreeID))
	if err := clickOnAgreeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnAgreeButton doesn't exist: ", err)
	} else if err := clickOnAgreeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAgreeButton: ", err)
	}

	// Click on startNoteTaking button.
	clickOnStartNoteTakingButton := d.Object(ui.ID(startNoteTakingID))
	if err := clickOnStartNoteTakingButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnStartNoteTakingButton doesn't exist: ", err)
	} else if err := clickOnStartNoteTakingButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnStartNoteTakingButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
