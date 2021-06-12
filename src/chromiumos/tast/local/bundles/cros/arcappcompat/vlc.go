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

// clamshellLaunchForVLC launches VLC in clamshell mode.
var clamshellLaunchForVLC = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForVLC},
}

// touchviewLaunchForVLC launches VLC in tablet mode.
var touchviewLaunchForVLC = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForVLC},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VLC,
		Desc:         "Functional test for VLC that install, launch the app and check that the main page is open, also checks VLC correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForVLC,
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
				Tests:      touchviewLaunchForVLC,
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
				Tests:      clamshellLaunchForVLC,
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
				Tests:      touchviewLaunchForVLC,
				CommonTest: testutil.TouchviewCommonTests,
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

// VLC test uses library for opting into the playstore and installing app.
// Checks VLC correctly changes the window states in both clamshell and touchview mode.
func VLC(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "org.videolan.vlc"
		appActivity = ".StartActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForVLC verifies VLC is launched and
// verify VLC reached main activity page of the app.
func launchAppForVLC(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowText = "ALLOW"
		doneText  = "DONE"
		nextID    = "org.videolan.vlc:id/next"
		noText    = "NO"
	)
	// Click on next Button.
	clickOnNextButton := d.Object(ui.ID(nextID))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}
	// Click on allow button.
	clickOnAllowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowText))
	if err := clickOnAllowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnAllowButton doesn't exist: ", err)
	} else if err := clickOnAllowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAllowButton: ", err)
	}

	// Click on next button.
	clickOnNextButton = d.Object(ui.ID(nextID))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}

	// Click on done button.
	clickOnDoneButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(doneText))
	if err := clickOnDoneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnDoneButton doesn't exist: ", err)
	} else if err := clickOnDoneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnDoneButton: ", err)
	}

	// Click on no button.
	clickOnNoButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(noText))
	if err := clickOnNoButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoButton doesn't exist: ", err)
	} else if err := clickOnNoButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
