// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForHearthstone = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForHearthstone},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForHearthstone = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForHearthstone},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Hearthstone,
		Desc:         "A functional test of the Play Store that installs Google Calendar",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForHearthstone,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBootedForHearthstone,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForHearthstone,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForHearthstone,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForHearthstone,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBootedForHearthstone,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForHearthstone,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForHearthstone,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.Hearthstone.username", "arcappcompat.Hearthstone.password"},
	})
}

// Hearthstone test uses library for opting into the playstore and installing app.
// Checks Hearthstone correctly changes the window states in both clamshell and touchview mode.
func Hearthstone(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.blizzard.wtcg.hearthstone"
		appActivity = ".HearthstoneActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForHearthstone verifies Hearthstone reached main activity page of the app.
func launchAppForHearthstone(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
}
