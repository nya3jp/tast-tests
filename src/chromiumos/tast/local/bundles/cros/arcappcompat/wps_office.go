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

// ClamshellTests are placed here.
var clamshellTestsForWPSOffice = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForWPSOffice},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForWPSOffice = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForWPSOffice},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WPSOffice,
		Desc:         "Functional test for WPSOffice that installs the app also verifies it is logged in and that the main page is open, checks WPSOffice correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("elm")),
		Params: []testing.Param{{
			Val:               clamshellTestsForWPSOffice,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForWPSOffice,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForWPSOffice,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForWPSOffice,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// WPSOffice test uses library for opting into the playstore and installing app.
// Checks  WPSOffice correctly changes the window states in both clamshell and touchview mode.
func WPSOffice(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "cn.wps.moffice_eng"
		appActivity = "cn.wps.moffice.documentmanager.PreStartActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForWPSOffice verifies app is logged in and
// verify app reached main activity page of the app.
func launchAppForWPSOffice(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		agreeText = "AGREE"
		agreeID   = "cn.wps.moffice_eng:id/start_page_agree_btn"
		startText = "Start WPS Office"
		allowText = "ALLOW"
		homeID    = "cn.wps.moffice_eng:id/home_my_roaming_userinfo_pic"
	)

	// Click on agree button.
	agreeButton := d.Object(ui.Text(agreeText), ui.ID(agreeID))
	if err := agreeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log(" agree button doesn't exists: ", err)
	} else if err := agreeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on agree button: ", err)
	}

	// Click on start button.
	startButton := d.Object(ui.Text(startText))
	if err := startButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log(" start button doesn't exists: ", err)
	} else if err := startButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on start button: ", err)
	}

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.Text(allowText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log(" allow button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allow button: ", err)
	}

	// Check home page is launched.
	homeButton := d.Object(ui.ID(homeID))
	if err := homeButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("home button doesn't exists: ", err)
	}
}
