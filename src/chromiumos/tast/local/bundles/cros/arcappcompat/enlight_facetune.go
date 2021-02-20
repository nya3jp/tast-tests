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
var clamshellTestsForEnlightFacetune = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForEnlightFacetune},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForEnlightFacetune = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForEnlightFacetune},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnlightFacetune,
		Desc:         "Functional test for EnlightFacetune that installs the app also verifies that the main page is open, checks EnlightFacetune correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForEnlightFacetune,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForEnlightFacetune,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForEnlightFacetune,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForEnlightFacetune,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// EnlightFacetune test uses library for opting into the playstore and installing app.
// Checks EnlightFacetune correctly changes the window states in both clamshell and touchview mode.
func EnlightFacetune(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.lightricks.facetune.free"
		appActivity = "com.lightricks.facetune.SplashActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForEnlightFacetune verifies EnlightFacetune reached main activity page of the app.
func launchAppForEnlightFacetune(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		closeID           = "com.lightricks.facetune.free:id/subscribe_skip"
		getStartedText    = "Get Started"
		shortTimeInterval = 300 * time.Millisecond
	)

	// Press KEYCODE_DPAD_RIGHT to swipe until getStarted button exist.
	getStartedButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+getStartedText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := getStartedButton.Exists(ctx); err != nil {
			s.Log(" Press KEYCODE_DPAD_RIGHT to swipe until getStartedText button exist")
			d.PressKeyCode(ctx, ui.KEYCODE_DPAD_RIGHT, 0)
			d.WaitForIdle(ctx, shortTimeInterval)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("getStartedButton doesn't exist: ", err)
	} else if err := getStartedButton.Click(ctx); err != nil {
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

	// Check for homeIcon on homePage.
	homeIcon := d.Object(ui.PackageName(appPkgName))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("homeIcon doesn't exists: ", err)
	}
}
