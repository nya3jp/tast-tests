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
var clamshellTestsForPhotolemur = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForPhotolemur},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForPhotolemur = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForPhotolemur},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Photolemur,
		Desc:         "Functional test for Photolemur that installs the app also verifies it is logged in and that the main page is open, checks Photolemur correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForPhotolemur,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBootedForPhotolemur,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForPhotolemur,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForPhotolemur,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForPhotolemur,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBootedForPhotolemur,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForPhotolemur,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForPhotolemur,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.Photolemur.username", "arcappcompat.Photolemur.password"},
	})
}

// Photolemur test uses library for opting into the playstore and installing app.
// Checks Photolemur correctly changes the window states in both clamshell and touchview mode.
func Photolemur(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.photolemur"
		appActivity = ".ui.activities.MainActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForPhotolemur verifies Photolemur is logged in and
// verify Photolemur reached main activity page of the app.
func launchAppForPhotolemur(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText = "ALLOW"
		homeIconText    = "OPEN"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Check for homeIcon on homePage.
	homeIcon := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+homeIconText))
	if err := homeIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("homeIcon doesn't exists: ", err)
	}
}
