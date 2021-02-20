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
var clamshellTestsForEdjingMix = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForEdjingMix},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForEdjingMix = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForEdjingMix},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EdjingMix,
		Desc:         "Functional test for EdjingMix that installs the app also verifies it is logged in and that the main page is open, checks EdjingMix correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForEdjingMix,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForEdjingMix,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForEdjingMix,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForEdjingMix,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// EdjingMix test uses library for opting into the playstore and installing app.
// Checks EdjingMix correctly changes the window states in both clamshell and touchview mode.
func EdjingMix(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.edjing.edjingdjturntable"
		appActivity = ".activities.PlatineActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForEdjingMix verifies EdjingMix is logged in and
// verify EdjingMix reached main activity page of the app.
func launchAppForEdjingMix(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		nextText       = "Next"
		skipText       = "SKIP"
		tryForFreeText = "TRY FOR FREE NOW"
		notNowText     = "NOT NOW"
		closeClassName = "android.widget.ImageView"
		allowText      = "ALLOW"
		allowID        = "com.android.packageinstaller:id/permission_allow_button"
	)
	var pressAllowEnabled bool
	// Click on next button until skip button exists.
	nextButton := d.Object(ui.Text(nextText))
	skipButton := d.Object(ui.Text(skipText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := skipButton.Exists(ctx); err != nil {
			s.Log(" Click on next button until skip button exist")
			nextButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("nextButton doesn't exist: ", err)
		pressAllowKeysEdjingMix(ctx, s, d)
		// To bypass pressKeysEdjingMix()
		pressAllowEnabled = true

	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip button: ", err)
	}

	if !pressAllowEnabled {
		// Click on skip button until try for free button exists.
		tryForFreeButton := d.Object(ui.Text(tryForFreeText))
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := tryForFreeButton.Exists(ctx); err != nil {
				s.Log(" Click on skip button until tryForFree button exist")
				skipButton.Click(ctx)
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
			s.Log("try for free doesn't exist: ", err)
		}
		pressKeysEdjingMix(ctx, s, d)
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	}
	pressAllowKeysEdjingMix(ctx, s, d)

	// Click on skip button
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Skip button doesn't exists: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip button: ", err)
	}

	// Check for homeIcon on homePage.
	homeIcon := d.Object(ui.PackageName(appPkgName))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("homeIcon doesn't exists: ", err)
	}
}

// pressAllowKeysEdjingMix runs the same set of keys twice to close the pop-up windows and land on home page
func pressAllowKeysEdjingMix(ctx context.Context, s *testing.State, d *ui.Device) {
	var count = 1

	for i := 0; i <= count; i++ {

		//Press tab and enter keys
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		} else {
			s.Log("Entered KEYCODE_TAB")
		}

		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		} else {
			s.Log("Entered KEYCODE_TAB")
		}

		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	}
}

// pressKeysEdjingMix runs a set of keys once to close ad dialog box
func pressKeysEdjingMix(ctx context.Context, s *testing.State, d *ui.Device) {
	var count = 1

	for i := 1; i <= count; i++ {

		//Press tab and enter keys
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		} else {
			s.Log("Entered KEYCODE_TAB")
		}

		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	}
}
