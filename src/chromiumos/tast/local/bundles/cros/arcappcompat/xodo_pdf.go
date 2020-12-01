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
var clamshellTestsForXodoPdf = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForXodoPdf},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForXodoPdf = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForXodoPdf},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         XodoPdf,
		Desc:         "Functional test for XodoPdf that installs the app also verifies it is logged in and that the main page is open, checks XodoPdf correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForXodoPdf,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForXodoPdf,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForXodoPdf,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForXodoPdf,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.XodoPdf.emailid", "arcappcompat.XodoPdf.password"},
	})
}

// XodoPdf test uses library for opting into the playstore and installing app.
// Checks XodoPdf correctly changes the window states in both clamshell and touchview mode.
func XodoPdf(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.xodo.pdf.reader"
		appActivity = "viewer.CompleteReaderMainActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForXodoPdf verifies XodoPdf is logged in and
// verify XodoPdf reached main activity page of the app.
func launchAppForXodoPdf(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowText         = "ALLOW"
		drawerDescription = "Open navigation drawer"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.Text(allowText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log(" allow button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allow button: ", err)
	}

	// Check for navigation drawer button.
	drawerButton := d.Object(ui.Description(drawerDescription))
	if err := drawerButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("Drawer button doesn't exist: ", err)
	}
}
