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
)

// ClamshellTests are placed here.
var clamshellTestsForVLC = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForVLC},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForVLC = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForVLC},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VLC,
		Desc:         "Functional test for VLC that install, launch the app and check that the main page is open, also checks VLC correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForVLC,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForVLC,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForVLC,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForVLC,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// VLC test uses library for opting into the playstore and installing app.
// Checks VLC correctly changes the window states in both clamshell and touchview mode.
func VLC(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "org.videolan.vlc"
		appActivity = ".StartActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForVLC verifies VLC is launched and
// verify VLC reached main activity page of the app.
func launchAppForVLC(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowText = "ALLOW"
		doneText  = "DONE"
		nextID    = "org.videolan.vlc:id/next"
		noText    = "NO"
		homeID    = "org.videolan.vlc:id/button_nomedia"
	)
	// Click on next Button.
	clickOnNextButton := d.Object(ui.ID(nextID))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}
	// Click on allow button.
	clickOnAllowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowText))
	if err := clickOnAllowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnAllowButton doesn't exist: ", err)
	} else if err := clickOnAllowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAllowButton: ", err)
	}

	// Click on next button.
	clickOnNextButton = d.Object(ui.ID(nextID))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}

	// Click on done button.
	clickOnDoneButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(doneText))
	if err := clickOnDoneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnDoneButton doesn't exist: ", err)
	} else if err := clickOnDoneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnDoneButton: ", err)
	}

	// Click on no button.
	clickOnNoButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(noText))
	if err := clickOnNoButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoButton doesn't exist: ", err)
	} else if err := clickOnNoButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoButton: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("homeIcon doesn't exist: ", err)
	}
}
