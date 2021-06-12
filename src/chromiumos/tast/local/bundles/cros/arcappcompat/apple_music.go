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

// clamshellLaunchForAppleMusic launches AppleMusic in clamshell mode.
var clamshellLaunchForAppleMusic = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForAppleMusic},
}

// touchviewLaunchForAppleMusic launches AppleMusic in tablet mode.
var touchviewLaunchForAppleMusic = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForAppleMusic},
}

// clamshellAppSpecificTestsForAppleMusic are placed here.
var clamshellAppSpecificTestsForAppleMusic = []testutil.TestSuite{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForAppleMusic are placed here.
var touchviewAppSpecificTestsForAppleMusic = []testutil.TestSuite{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppleMusic,
		Desc:         "Functional test for AppleMusic that installs the app also verifies that the main page is open, checks AppleMusic correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForAppleMusic,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForAppleMusic,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForAppleMusic,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForAppleMusic,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove ExtraHardwareDeps once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForAppleMusic,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForAppleMusic,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForAppleMusic,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForAppleMusic,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove ExtraHardwareDeps once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// AppleMusic test uses library for opting into the playstore and installing app.
// Checks AppleMusic correctly changes the window states in both clamshell and touchview mode.
func AppleMusic(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.apple.android.music"
		appActivity = ".onboarding.activities.SplashActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForAppleMusic verifies AppleMusic reached main activity page of the app.
func launchAppForAppleMusic(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		agreeButtonText       = "Agree"
		continueButtonText    = "Continue"
		scrollLayoutClassName = "android.webkit.WebView"
	)

	// Check for scroll layout.
	scrollLayout := d.Object(ui.ClassName(scrollLayoutClassName), ui.Scrollable(true))
	if err := scrollLayout.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("scrollLayout doesn't exist: ", err)
	}
	// Scroll until Agree button is enabled.
	agreeButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+agreeButtonText), ui.Enabled(true))
	if err := scrollLayout.ScrollTo(ctx, agreeButton); err != nil {
		s.Log("Scrolled until agree button is enabled")
	} else {
		s.Log("Scroll does not happen: ", err)
	}

	if err := agreeButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("agreeButton doesn't exist: ", err)
	} else if err := agreeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on agreeButton: ", err)
	}

	// Click on continue button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueButtonText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
