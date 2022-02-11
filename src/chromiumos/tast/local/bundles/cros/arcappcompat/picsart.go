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

// clamshellLaunchForPicsart launches Picsart in clamshell mode.
var clamshellLaunchForPicsart = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForPicsart},
}

// touchviewLaunchForPicsart launches Picsart in tablet mode.
var touchviewLaunchForPicsart = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForPicsart},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Picsart,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Picsart that install, launch the app and check that the main page is open, also checks Picsart correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		// Disabled the test since Picsart is not compatible with Chromebook.
		// Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForPicsart,
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
				LaunchTests: touchviewLaunchForPicsart,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForPicsart,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForPicsart,
				CommonTests: testutil.TouchviewCommonTests,
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

// Picsart test uses library for opting into the playstore and installing app.
// Checks Picsart correctly changes the window states in both clamshell and touchview mode.
func Picsart(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.picsart.studio"
		appActivity = "com.socialin.android.photo.picsinphoto.MainPagerActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForPicsart verifies Picsart is launched and
// verify Picsart reached main activity page of the app.
func launchAppForPicsart(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowText      = "ALLOW"
		closeClassName = "android.widget.ImageButton"
		closeDes       = "Navigate up"
		homeText       = "Home"
		skipID         = "com.picsart.studio:id/btnSkip"
	)
	// Click on skip button.
	skipButton := d.Object(ui.ID(skipID))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("skipButton doesn't exists: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}

	// Click on allow button.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on close button.
	closeButton := d.Object(ui.ClassName(closeClassName), ui.Description(closeDes))
	if err := closeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("closeButton doesn't exists: ", err)
	} else if err := closeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on closeButton: ", err)
	}

	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
