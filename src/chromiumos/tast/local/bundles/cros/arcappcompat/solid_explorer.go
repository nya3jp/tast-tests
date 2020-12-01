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
var clamshellTestsForSolidExplorer = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSolidExplorer},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForSolidExplorer = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSolidExplorer},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SolidExplorer,
		Desc:         "Functional test for SolidExplorer that installs the app also verifies it is logged in and that the main page is open, checks SolidExplorer correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForSolidExplorer,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForSolidExplorer,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForSolidExplorer,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForSolidExplorer,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// SolidExplorer test uses library for opting into the playstore and installing app.
// Checks SolidExplorer correctly changes the window states in both clamshell and touchview mode.
func SolidExplorer(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "pl.solidexplorer2"
		appActivity = "pl.solidexplorer.SolidExplorer"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForSolidExplorer verifies SolidExplorer is logged in and
// verify SolidExplorer reached main activity page of the app.
func launchAppForSolidExplorer(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		skipID    = "pl.solidexplorer2:id/btn_skip"
		licenceID = "pl.solidexplorer2:id/cb_license"
		gotItText = "GOT IT"
		doneID    = "pl.solidexplorer2:id/btn_next"
		allowText = "ALLOW"
		OkText    = "OK"
		drawerID  = "pl.solidexplorer2:id/ab_icon"
	)

	// Click on skip button.
	skipButton := d.Object(ui.ID(skipID))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Skip button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip button: ", err)
	}

	// Click on licence button.
	licenceButton := d.Object(ui.ID(licenceID))
	if err := licenceButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Licence button doesn't exist: ", err)
	} else if err := licenceButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on licence button: ", err)
	}

	// Click on got it button.
	gotItButton := d.Object(ui.Text(gotItText))
	if err := gotItButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Got It button doesn't exist: ", err)
	} else if err := gotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Got It button: ", err)
	}

	// Click on done button.
	doneButton := d.Object(ui.ID(doneID))
	if err := doneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Done button doesn't exist: ", err)
	} else if err := doneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on done button: ", err)
	}

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.Text(allowText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log(" allow button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allow button: ", err)
	}

	// Click on OK button.
	OKButton := d.Object(ui.Text(OkText))
	if err := OKButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Ok button doesn't exist: ", err)
	} else if err := OKButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Ok button: ", err)
	}

	// Check for navigation drawer button.
	drawerButton := d.Object(ui.ID(drawerID))
	if err := drawerButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("Drawer button doesn't exist: ", err)
	}
}
