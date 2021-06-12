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

// clamshellLaunchForKine launches the app.
var clamshellLaunchForKine = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForKinemaster},
}

// touchviewLaunchForKine launches the app.
var touchviewLaunchForKine = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForKinemaster},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Kinemaster,
		Desc:         "Functional test for Kinemaster that installs the app also verifies it is logged in and that the main page is open, checks Kinemaster correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForKine,
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
				Tests:      touchviewLaunchForKine,
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
				Tests:      clamshellLaunchForKine,
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
				Tests:      touchviewLaunchForKine,
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
		addButtonID             = "com.nexstreaming.app.kinemasterfree:id/addProject"
		allowButtonText         = "ALLOW"
		cancelText              = "Cancel"
		continueToAppID         = "close-button"
		closeID                 = "com.nexstreaming.app.kinemasterfree:id/skip_ad_button"
		closeButtonID           = "com.nexstreaming.app.kinemasterfree:id/collapse_button"
		noText                  = "No"
		okText                  = "OK"
		homeID                  = "com.nexstreaming.app.kinemasterfree:id/mediaListView"
		remindMelaterButtonText = "Remind Me Later"
		startText               = "Start"
		selectLayoutID          = "com.nexstreaming.app.kinemasterfree:id/ratio16v9"
		shortTimeInterval       = 300 * time.Millisecond
	)

	// Click on OK Button.
	clickOnOkButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+okText))
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

	// Click on no button.
	clickOnNoButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+noText))
	if err := clickOnNoButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnNoButton doesn't exists: ", err)
	} else if err := clickOnNoButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoButton: ", err)
	}

	// Click on continue to app button.
	continueToAppButton := d.Object(ui.ID(continueToAppID))
	if err := continueToAppButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("continueToAppButton doesn't exists: ", err)
	} else if err := continueToAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueToAppButton: ", err)
	}

	// Click on remind me later button.
	clickOnRemindMeLaterButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+remindMelaterButtonText))
	if err := clickOnRemindMeLaterButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnRemindMeLaterButton doesn't exists: ", err)
	} else if err := clickOnRemindMeLaterButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnRemindMeLaterButton: ", err)
	}

	// Click on cancel button.
	cancelButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+cancelText))
	if err := cancelButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("cancelButton doesn't exists: ", err)
	} else if err := cancelButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on cancelButton: ", err)
	}

	// Click on close button.
	clickOnCloseButton = d.Object(ui.ID(closeButtonID))
	if err := clickOnCloseButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnCloseButton doesn't exists: ", err)
	} else if err := clickOnCloseButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnCloseButton: ", err)
	}

	// Click on add button.
	clickOnAddButton := d.Object(ui.ID(addButtonID))
	if err := clickOnAddButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnAddButton doesn't exists: ", err)
	} else if err := clickOnAddButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAddButton: ", err)
	}

	// Click on select Layout.
	clickOnSlectLayout := d.Object(ui.ID(selectLayoutID))
	if err := clickOnSlectLayout.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnSlectLayout doesn't exists: ", err)
	} else if err := clickOnSlectLayout.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnSlectLayout: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
