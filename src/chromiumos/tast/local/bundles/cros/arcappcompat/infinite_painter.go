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
var clamshellTestsForInfinitePainter = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForInfinitePainter},
	{Name: "Clamshell: Mouse Scroll", Fn: testutil.MouseScrollAction},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForInfinitePainter = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForInfinitePainter},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         InfinitePainter,
		Desc:         "Functional test for InfinitePainter that installs the app also verifies it is logged in and that the main page is open, checks InfinitePainter correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForInfinitePainter,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForInfinitePainter,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForInfinitePainter,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForInfinitePainter,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// InfinitePainter test uses library for opting into the playstore and installing app.
// Checks InfinitePainter correctly changes the window states in both clamshell and touchview mode.
func InfinitePainter(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.brakefield.painter"
		appActivity = ".activities.ActivityStartup"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForInfinitePainter verifies InfinitePainter is logged in and
// verify InfinitePainter reached main activity page of the app.
func launchAppForInfinitePainter(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowText          = "ALLOW"
		optionsDescription = "Options"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.Text(allowText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log(" allow button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allow button: ", err)
	}

	// Press back key until option button exist.
	optionsButton := d.Object(ui.Description(optionsDescription))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := optionsButton.Exists(ctx); err != nil {
			if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
				s.Log("Failed to press BACK_CODE: ", err)
			} else {
				s.Log("BACK_CODE pressed")
			}
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("Options button doesn't exist: ", err)
	}

}
