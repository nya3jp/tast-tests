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
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
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
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForHearthstone,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForHearthstone,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForHearthstone,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
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
		appActivity = "com.blizzard.wtcg.hearthstone.HearthstoneActivity"
	)

	username := s.RequiredVar("arcappcompat.Hearthstone.username")
	password := s.RequiredVar("arcappcompat.Hearthstone.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"),
		chrome.ARCSupported(), chrome.ExtraArgs("--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()
	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			}
			if err := a.PullFile(ctx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()
	defer func() {
		if s.HasError() {
			d.Close()
		}
	}()
	s.Log("Enable showing ANRs")
	if err := a.Command(ctx, "settings", "put", "secure", "anr_show_background", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable showing ANRs: ", err)
	}
	s.Log("Enable crash dialog")
	if err := a.Command(ctx, "settings", "put", "secure", "show_first_crash_dialog_dev_option", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable crash dialog: ", err)
	}

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, appPkgName); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	testSet := s.Param().(testutil.TestParams)
	// Run the different test cases.
	for idx, test := range testSet.Tests {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			defer func() {
				if s.HasError() {
					path := fmt.Sprintf("%s/screenshot-arcappcompat-failed-test-%d.png", s.OutDir(), idx)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
				}
			}()
			// Launch the app.
			act, err := arc.NewActivity(a, appPkgName, appActivity)
			if err != nil {
				s.Fatal("Failed to create new app activity: ", err)
			}
			s.Log("Created new app activity")

			defer act.Close()
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed start app: ", err)
			}
			s.Log("App launched successfully")

			defer act.Stop(ctx)

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
