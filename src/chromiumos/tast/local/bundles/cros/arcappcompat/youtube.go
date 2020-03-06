// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForYoutube = []testutil.TestSuite{
	{"Launch app in Clamshell", launchAppForYoutube},
	{"Clamshell: Fullscreen app", testutil.ClamshellFullscreenApp},
	{"Clamshell: Minimise and Restore", testutil.MinimizeRestoreApp},
	{"Clamshell: Resize window", testutil.ClamshellResizeWindow},
	{"Clamshell: Reopen app", testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForYoutube = []testutil.TestSuite{
	{"Launch app in Touchview", launchAppForYoutube},
	{"Touchview: Minimise and Restore", testutil.MinimizeRestoreApp},
	{"Touchview: Reopen app", testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Youtube,
		Desc:         "Functional test for Youtube  that installs the app also verifies it is logged in and that the main page is open, checks Gmail correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				false,
				clamshellTestsForYoutube,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedAppCompat(),
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				true,
				touchviewTestsForYoutube,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				false,
				clamshellTestsForYoutube,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				true,
				touchviewTestsForYoutube,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               arc.VMBootedInTabletModeAppCompat(),
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Youtube test uses library for opting into the playstore and installing app.
// Checks Youtube correctly changes the window states in both clamshell and touchview mode.
func Youtube(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.youtube"
		appActivity = "com.google.android.apps.youtube.app.WatchWhileActivity"

		openButtonRegex = "Open|OPEN"
	)

	// Setup Chrome.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Enable showing ANRs")
	if err := a.Command(ctx, "settings", "put", "secure", "anr_show_background", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable showing ANRs: ", err)
	}
	s.Log("Enable crash dialog")
	if err := a.Command(ctx, "settings", "put", "secure", "show_first_crash_dialog_dev_option", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable crash dialog: ", err)
	}

	s.Log("Installing app")
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	s.Log("Launch the app")
	testutil.LaunchHelper(ctx, s, d)

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
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// Verify app is logged in.
// Verify app reached main activity page of the app.
func launchAppForYoutube(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	const (
		accountImageDescription = "Account"
		accountImageID          = "com.google.android.youtube:id/image"
		openButtonClassName     = "android.widget.Button"
		openButtonRegex         = "Open|OPEN"

		longUITimeout = 5 * time.Minute
	)

	// Check for account image in home page.
	accountImage := d.Object(ui.ID(accountImageID), ui.DescriptionContains(accountImageDescription))
	testutil.FindTheError(s, accountImage.WaitForExists(ctx, testutil.LongUITimeout))
}
