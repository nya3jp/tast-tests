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

// clamshellLaunchForEnlightFacetune launches EnlightFacetune in clamshell mode.
var clamshellLaunchForEnlightFacetune = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForEnlightFacetune},
}

// touchviewLaunchForEnlightFacetune launches EnlightFacetune in tablet mode.
var touchviewLaunchForEnlightFacetune = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForEnlightFacetune},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnlightFacetune,
		Desc:         "Functional test for EnlightFacetune that installs the app also verifies that the main page is open, checks EnlightFacetune correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:       clamshellLaunchForEnlightFacetune,
				ARCPSpecificTests: testutil.ClamshellARCPCommonTests,
				CommonTests:       testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForEnlightFacetune,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:     clamshellLaunchForEnlightFacetune,
				VMSpecificTests: testutil.ClamshellVMCommonTests,
				CommonTests:     testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForEnlightFacetune,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// EnlightFacetune test uses library for opting into the playstore and installing app.
// Checks EnlightFacetune correctly changes the window states in both clamshell and touchview mode.
func EnlightFacetune(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.lightricks.facetune.free"
		appActivity = "com.lightricks.facetune.SplashActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForEnlightFacetune verifies EnlightFacetune reached main activity page of the app.
func launchAppForEnlightFacetune(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		loginWithGoogleButtonText = "Sign in with Google"
		closeID                   = "com.lightricks.facetune.free:id/subscribe_skip"
		emailAddressID            = "com.google.android.gms:id/container"
		getStartedText            = "Get Started"
		shortTimeInterval         = 300 * time.Millisecond
	)

	// Press KEYCODE_DPAD_RIGHT to swipe until getStarted button exist.
	getStartedButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+getStartedText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := getStartedButton.Exists(ctx); err != nil {
			s.Log(" Press KEYCODE_DPAD_RIGHT to swipe until getStartedText button exist")
			d.PressKeyCode(ctx, ui.KEYCODE_DPAD_RIGHT, 0)
			d.WaitForIdle(ctx, shortTimeInterval)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("getStartedButton doesn't exist: ", err)
	} else if err := getStartedButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on startButton: ", err)
	}

	loginWithGoogleButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(loginWithGoogleButtonText))
	emailAddress := d.Object(ui.ID(emailAddressID))
	// Click on sign in with Google Button until email address exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := emailAddress.Exists(ctx); err != nil {
			loginWithGoogleButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("Email address doesn't exist: ", err)
	}

	// Click on email address.
	if err := emailAddress.Exists(ctx); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	}

	// Click on close button to skip subscription.
	clickOnCloseButton := d.Object(ui.ID(closeID))
	if err := clickOnCloseButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnCloseButton doesn't exists: ", err)
		d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0)
	} else if err := clickOnCloseButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnCloseButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
