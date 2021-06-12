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

// clamshellLaunchForGooglePhotos launches GooglePhotos in clamshell mode.
var clamshellLaunchForGooglePhotos = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForGooglePhotos},
}

// touchviewLaunchForGooglePhotos launches GooglePhotos in tablet mode.
var touchviewLaunchForGooglePhotos = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForGooglePhotos},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GooglePhotos,
		Desc:         "Functional test for GooglePhotos that installs the app also verifies it is logged in and that the main page is open, checks GooglePhotos correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForGooglePhotos,
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
				Tests:      touchviewLaunchForGooglePhotos,
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
				Tests:      clamshellLaunchForGooglePhotos,
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
				Tests:      touchviewLaunchForGooglePhotos,
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

// GooglePhotos test uses library for opting into the playstore and installing app.
// Checks GooglePhotos correctly changes the window states in both clamshell and touchview mode.
func GooglePhotos(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.apps.photos"
		appActivity = ".home.HomeActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForGooglePhotos verifies GooglePhotos is logged in and
// verify GooglePhotos reached main activity page of the app.
func launchAppForGooglePhotos(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText   = "ALLOW"
		confirmButtonText = "Confirm"
		skipButtonID      = "com.google.android.apps.photos:id/welcomescreens_skip_button"
		turnOnBackUpText  = "Turn on Backup"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Click on turn on back up button.
	turnOnBackUpButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(turnOnBackUpText))
	if err := turnOnBackUpButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("TurnOn BackUp Button doesn't exist: ", err)
	} else if err := turnOnBackUpButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on turnOnBackUpButton: ", err)
	}
	// Click on skip button.
	skipButton := d.Object(ui.ID(skipButtonID))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Skip Button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}
	// Click on confirm button.
	confirmButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(confirmButtonText))
	if err := confirmButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Confirm Button doesn't exist: ", err)
	} else if err := confirmButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on confirmButton: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
