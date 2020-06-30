// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappcompat

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForHearthstone = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForHearthstone},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForHearthstone = []testutil.TestSuite{
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
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForHearthstone,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBootedForHearthstone,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForHearthstone,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletModeForHearthstone,
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForHearthstone,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBootedForHearthstone,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForHearthstone,
			},
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
	// Step up chrome on Chromebook.
	cr, tconn, a, d := testutil.SetUpDevice(ctx, s, appPkgName, appActivity)
	defer d.Close()

	// Ensure app launches before test cases.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app before test cases: ", err)
	}
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app before test cases: ", err)
	}

	testSet := s.Param().(testutil.TestParams)
	// Run the different test cases.
	for idx, test := range testSet.Tests {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			// Launch the app.
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			defer act.Stop(ctx, tconn)

			defer func() {
				if s.HasError() {
					filename := fmt.Sprintf("screenshot-arcappcompat-failed-test-%d.png", idx)
					path := filepath.Join(s.OutDir(), filename)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
				}
			}()

			testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForHearthstone verifies Hearthstone reached main activity page of the app.
func launchAppForHearthstone(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	if currentAppPkg := testutil.CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("Failed to launch the app: ", currentAppPkg)
	}
	s.Log("App is launched successfully in launchAppForHearthstone")
}
