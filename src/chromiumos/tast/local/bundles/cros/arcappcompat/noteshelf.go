// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForNoteshelf = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForNoteshelf},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForNoteshelf = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForNoteshelf},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Noteshelf,
		Desc:         "Functional test for Noteshelf that install, launch the app and check that the main page is open, also checks Noteshelf correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForNoteshelf,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBootedForNoteshelf,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForNoteshelf,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForNoteshelf,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForNoteshelf,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBootedForNoteshelf,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForNoteshelf,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForNoteshelf,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.Noteshelf.username", "arcappcompat.Noteshelf.password"},
	})
}

// Noteshelf test uses library for opting into the playstore and installing app.
// Checks Noteshelf correctly changes the window states in both clamshell and touchview mode.
func Noteshelf(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.fluidtouch.noteshelf2"
		appActivity = "com.fluidtouch.noteshelf.commons.ui.FTSplashScreenActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForNoteshelf verifies Noteshelf is launched and
// verify Noteshelf reached main activity page of the app.
func launchAppForNoteshelf(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		agreeID           = "com.fluidtouch.noteshelf2:id/agreeCheckLayout"
		continueButtonID  = "com.fluidtouch.noteshelf2:id/welcome_screen1_continue_button"
		homeID            = "com.fluidtouch.noteshelf2:id/menu_create_notebook"
		skipText          = "SKIP"
		startNoteTakingID = "com.fluidtouch.noteshelf2:id/welcome_screen5_start_button"
	)
	// Click on continue button.
	clickOnContinueButton := d.Object(ui.ID(continueButtonID))
	if err := clickOnContinueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnContinueButton doesn't exist: ", err)
	} else if err := clickOnContinueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnContinueButton: ", err)
	}

	// Click on skip button.
	clickOnSkipButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(skipText))
	if err := clickOnSkipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnSkipButton doesn't exist: ", err)
	} else if err := clickOnSkipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnSkipButton: ", err)
	}

	// Click on agree button.
	clickOnAgreeButton := d.Object(ui.ID(agreeID))
	if err := clickOnAgreeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnAgreeButton doesn't exist: ", err)
	} else if err := clickOnAgreeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAgreeButton: ", err)
	}

	// Click on startNoteTaking button.
	clickOnStartNoteTakingButton := d.Object(ui.ID(startNoteTakingID))
	if err := clickOnStartNoteTakingButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnStartNoteTakingButton doesn't exist: ", err)
	} else if err := clickOnStartNoteTakingButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnStartNoteTakingButton: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("homeIcon doesn't exist: ", err)
	}
}
