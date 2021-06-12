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

// clamshellLaunchForMyscriptNebo launches MyscriptNebo in clamshell mode.
var clamshellLaunchForMyscriptNebo = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForMyscriptNebo},
}

// touchviewLaunchForMyscriptNebo launches MyscriptNebo in tablet mode.
var touchviewLaunchForMyscriptNebo = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForMyscriptNebo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MyscriptNebo,
		Desc:         "Functional test for MyscriptNebo that installs the app also verifies it is logged in and that the main page is open, checks MyscriptNebo correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForMyscriptNebo,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedForMyscriptNebo,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForMyscriptNebo,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeForMyscriptNebo,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForMyscriptNebo,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedForMyscriptNebo,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForMyscriptNebo,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeForMyscriptNebo,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.MyscriptNebo.username", "arcappcompat.MyscriptNebo.password"},
	})
}

// MyscriptNebo test uses library for opting into the playstore and installing app.
// Checks MyscriptNebo correctly changes the window states in both clamshell and touchview mode.
func MyscriptNebo(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.myscript.nebo"
		appActivity = ".BootstrapActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForMyscriptNebo verifies MyscriptNebo is logged in and
// verify MyscriptNebo reached main activity page of the app.
func launchAppForMyscriptNebo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		agreeButtonText      = "I AGREE"
		nextID               = "com.myscript.nebo:id/onboarding_next_button"
		homeID               = "com.myscript.nebo:id/onboarding_pager"
		understandButtonText = "I UNDERSTAND"
	)

	// Click on agree button.
	agreeButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+agreeButtonText))
	if err := agreeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("agreeButton doesn't exist: ", err)
	} else if err := agreeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on agreeButton: ", err)
	}

	// Click on I understand button.
	clickOnIUnderstandButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+understandButtonText))
	if err := clickOnIUnderstandButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnIUnderstandButton doesn't exist: ", err)
	} else if err := clickOnIUnderstandButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnIUnderstandButton: ", err)
	}
	// Click on next button until home page exist.
	nextButton := d.Object(ui.ID(nextID))
	homePage := d.Object(ui.ID(homeID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := homePage.Exists(ctx); err != nil {
			s.Log(" Click on next button until home page exist")
			nextButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("homePage doesn't exist: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
