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

// clamshellLaunchForXodoPdf launches XodoPdf in clamshell mode.
var clamshellLaunchForXodoPdf = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForXodoPdf},
}

// touchviewLaunchForXodoPdf launches XodoPdf in tablet mode.
var touchviewLaunchForXodoPdf = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForXodoPdf},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         XodoPdf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for XodoPdf that installs the app also verifies it is logged in and that the main page is open, checks XodoPdf correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForXodoPdf,
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
				LaunchTests: touchviewLaunchForXodoPdf,
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
				LaunchTests: clamshellLaunchForXodoPdf,
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
				LaunchTests: touchviewLaunchForXodoPdf,
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

// XodoPdf test uses library for opting into the playstore and installing app.
// Checks XodoPdf correctly changes the window states in both clamshell and touchview mode.
func XodoPdf(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.xodo.pdf.reader"
		appActivity = "viewer.CompleteReaderMainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForXodoPdf verifies XodoPdf is logged in and
// verify XodoPdf reached main activity page of the app.
func launchAppForXodoPdf(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText = "ALLOW"
		continueBtnText = "CONTINUE TO APP"
		toggleBtnID     = "android:id/switch_widget"
		navigateDes     = "Back"
	)
	// Click on continue to app.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueBtnText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on allow button to access your photos, media and files.
	allowBtn := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Enable toggle button to allow access to manage all files.
	toggleBtn := d.Object(ui.ID(toggleBtnID))
	if err := toggleBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("toggleBtn doesn't exists: ", err)
	} else if err := toggleBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on toggleBtn: ", err)
	}

	// Click on navigate button.
	navigateBtn := d.Object(ui.DescriptionMatches("(?i)" + navigateDes))
	if err := navigateBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("navigateBtn doesn't exists: ", err)
	} else if err := navigateBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on navigateBtn: ", err)
	}

	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
