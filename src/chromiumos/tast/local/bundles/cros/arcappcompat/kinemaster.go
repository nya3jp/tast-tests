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

// clamshellLaunchForKine launches the app.
var clamshellLaunchForKine = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForKinemaster},
}

// touchviewLaunchForKine launches the app.
var touchviewLaunchForKine = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForKinemaster},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Kinemaster,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Kinemaster that installs the app also verifies it is logged in and that the main page is open, checks Kinemaster correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForKine,
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
				LaunchTests: touchviewLaunchForKine,
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
				LaunchTests: clamshellLaunchForKine,
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
				LaunchTests: touchviewLaunchForKine,
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

// Kinemaster test uses library for opting into the playstore and installing app.
// Checks Kinemaster correctly changes the window states in both clamshell and touchview mode.
func Kinemaster(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.nexstreaming.app.kinemasterfree"
		appActivity = "com.nextreaming.nexeditorui.MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForKinemaster verifies Kinemaster is logged in and
// verify Kinemaster reached main activity page of the app.
func launchAppForKinemaster(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText         = "ALLOW"
		cancelID                = "com.nexstreaming.app.kinemasterfree:id/app_dialog_button_left_container"
		closeID                 = "com.nexstreaming.app.kinemasterfree:id/ib_close_button"
		okText                  = "OK"
		remindMelaterButtonText = "Remind Me Later"
		startText               = "Start"
		shortTimeInterval       = 300 * time.Millisecond
	)

	// Click on OK Button.
	clickOnOkButton := d.Object(ui.TextMatches("(?i)" + okText))
	if err := clickOnOkButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnOkButton doesn't exists: ", err)
	} else if err := clickOnOkButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnOkButton: ", err)
	}

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
			s.Log(" Press KEYCODE_DPAD_RIGHT to swipe until Start button exist")
			d.PressKeyCode(ctx, ui.KEYCODE_DPAD_RIGHT, 0)
			d.WaitForIdle(ctx, shortTimeInterval)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("startButton doesn't exist: ", err)
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

	// Click on remind me later button.
	clickOnRemindMeLaterButton := d.Object(ui.TextMatches("(?i)" + remindMelaterButtonText))
	if err := clickOnRemindMeLaterButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnRemindMeLaterButton doesn't exists: ", err)
	} else if err := clickOnRemindMeLaterButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnRemindMeLaterButton: ", err)
	}

	// Click on cancel button.
	cancelButton := d.Object(ui.ID(cancelID))
	if err := cancelButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("cancelButton doesn't exists: ", err)
	} else if err := cancelButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on cancelButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
