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
var clamshellTestsForWevideo = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForWevideo},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForWevideo = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForWevideo},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Wevideo,
		Desc:         "Functional test for Wevideo that installs the app also verifies that the main page is open, checks Wevideo correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForWevideo,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForWevideo,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForWevideo,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForWevideo,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Wevideo test uses library for opting into the playstore and installing app.
// Checks Wevideo correctly changes the window states in both clamshell and touchview mode.
func Wevideo(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.wevideo.mobile.android"
		appActivity = ".ui.SplashActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForWevideo verifies Wevideo reached main activity page of the app.
func launchAppForWevideo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueButtonText = "CONTINUE"
		reviewButtonText   = "REVIEW"
		homeID             = "com.wevideo.mobile.android:id/create_new_timeline_fab"
	)
	// Click on review button to review private settings.
	reviewButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+reviewButtonText))
	if err := reviewButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("reviewButton doesn't exist: ", err)
	} else if err := reviewButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on reviewButton: ", err)
	}

	// Click on continue button to review private settings.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueButtonText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Check for homeIcon on homePage.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("homeIcon doesn't exists: ", err)
	}
}
