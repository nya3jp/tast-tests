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

// clamshellLaunchForArtrage launches Artrage in clamshell mode.
var clamshellLaunchForArtrage = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForArtrage},
}

// touchviewLaunchForArtrage launches Artrage in tablet mode.
var touchviewLaunchForArtrage = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForArtrage},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Artrage,
		Desc:         "Functional test for Artrage that installs the app also verifies it is logged in and that the main page is open, checks Artrage correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForArtrage,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedForArtrage,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForArtrage,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeForArtrage,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForArtrage,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedForArtrage,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForArtrage,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeForArtrage,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.Artrage.username", "arcappcompat.Artrage.password"},
	})
}

// Artrage test uses library for opting into the playstore and installing app.
// Checks Artrage correctly changes the window states in both clamshell and touchview mode.
func Artrage(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.ambientdesign.artrage.playstore"
		appActivity = "com.ambientdesign.artrage.MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForArtrage verifies Artrage is logged in and
// verify Artrage reached main activity page of the app.
func launchAppForArtrage(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText = "ALLOW"
		retryButtonText = "Retry"
		homeID          = "com.ambientdesign.artrage.playstore:id/ic_system"
	)

	// Click on allow button.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on retry button to check for licence.
	retryButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+retryButtonText))
	if err := retryButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("retryButton doesn't exist: ", err)
	}

	// Click on retry button to check for licence until home page exist.
	retryButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+retryButtonText))
	homePage := d.Object(ui.ID(homeID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := homePage.Exists(ctx); err != nil {
			s.Log(" Click on retry button until home page exist")
			retryButton.Click(ctx)
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
