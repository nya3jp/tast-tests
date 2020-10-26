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
var clamshellTestsForGachaLife = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForGachaLife},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForGachaLife = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForGachaLife},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GachaLife,
		Desc:         "Functional test for GachaLife that install, launch the app and check that the main page is open, also checks GachaLife correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForGachaLife,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForGachaLife,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForGachaLife,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForGachaLife,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GachaLife test uses library for opting into the playstore and installing app.
// Checks GachaLife correctly changes the window states in both clamshell and touchview mode.
func GachaLife(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "air.com.lunime.gachalife"
		appActivity = ".AppEntry"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForGachaLife verifies GachaLife is launched and
// verify GachaLife reached main activity page of the app.
func launchAppForGachaLife(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const homeClassName = "android.widget.FrameLayout"

	// Check for home icon.
	homeIcon := d.Object(ui.ClassName(homeClassName), ui.PackageName(appPkgName))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("homeIcon doesn't exist: ", err)
	}
}
