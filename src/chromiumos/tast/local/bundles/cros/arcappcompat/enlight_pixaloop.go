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

// clamshellLaunchForEnlightPixaloop launches EnlightPixaloop in clamshell mode.
var clamshellLaunchForEnlightPixaloop = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForEnlightPixaloop},
}

// touchviewLaunchForEnlightPixaloop launches EnlightPixaloop in tablet mode.
var touchviewLaunchForEnlightPixaloop = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForEnlightPixaloop},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnlightPixaloop,
		Desc:         "Functional test for EnlightPixaloop that installs the app also verifies that the main page is open, checks EnlightPixaloop correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForEnlightPixaloop,
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
				Tests:      touchviewLaunchForEnlightPixaloop,
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
				Tests:      clamshellLaunchForEnlightPixaloop,
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
				Tests:      touchviewLaunchForEnlightPixaloop,
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

// EnlightPixaloop test uses library for opting into the playstore and installing app.
// Checks EnlightPixaloop correctly changes the window states in both clamshell and touchview mode.
func EnlightPixaloop(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.lightricks.pixaloop"
		appActivity = ".SplashActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForEnlightPixaloop verifies EnlightPixaloop reached main activity page of the app.
func launchAppForEnlightPixaloop(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText   = "ALLOW"
		closeID           = "com.lightricks.pixaloop:id/subscribe_skip"
		startText         = "Dive right in!"
		shortTimeInterval = 300 * time.Millisecond
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Press KEYCODE_DPAD_RIGHT to swipe until Start button exist.
	startButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+startText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := startButton.Exists(ctx); err != nil {
			s.Log("Press KEYCODE_DPAD_RIGHT to swipe until Start button exist")
			d.PressKeyCode(ctx, ui.KEYCODE_DPAD_RIGHT, 0)
			d.WaitForIdle(ctx, shortTimeInterval)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Error("startButton doesn't exist: ", err)
	} else if err := startButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on startButton: ", err)
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
