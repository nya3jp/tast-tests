// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForGooglePhotos = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForGooglePhotos},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForGooglePhotos = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForGooglePhotos},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GooglePhotos,
		Desc:         "Functional test for GooglePhotos that installs the app also verifies it is logged in and that the main page is open, checks GooglePhotos correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForGooglePhotos,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedAppCompat(),
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForGooglePhotos,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForGooglePhotos,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForGooglePhotos,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               arc.VMBootedInTabletModeAppCompat(),
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GooglePhotos test uses library for opting into the playstore and installing app.
// Checks GooglePhotos correctly changes the window states in both clamshell and touchview mode.
func GooglePhotos(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.apps.photos"
		appActivity = "com.google.android.apps.photos.home.HomeActivity"
	)

	// Step up chrome on Chromebook.
	cr, tconn, a, d := testutil.SetUpDevice(ctx, s, appPkgName, appActivity)
	defer d.Close()

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
				s.Fatal("Failed to start app: ", err)
			}
			s.Log("App launched successfully")

			defer act.Stop(ctx)

			testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForGooglePhotos verifies GooglePhotos is logged in and
// verify GooglePhotos reached main activity page of the app.
func launchAppForGooglePhotos(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText   = "ALLOW"
		confirmButtonText = "Confirm"
		skipButtonID      = "com.google.android.apps.photos:id/welcomescreens_skip_button"
		turnOnBackUpText  = "Turn on Backup"
		photosIconID      = "com.google.android.apps.photos:id/tab_layout"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}
	// Click on turn on back up button.
	turnOnBackUpButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(turnOnBackUpText))
	if err := turnOnBackUpButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("TurnOn BackUp Button doesn't exist: ", err)
	} else if err := turnOnBackUpButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on turnOnBackUpButton: ", err)
	}
	// Click on skip button.
	skipButton := d.Object(ui.ID(skipButtonID))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Skip Button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}
	// Click on confirm button.
	confirmButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(confirmButtonText))
	if err := confirmButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Confirm Button doesn't exist: ", err)
	} else if err := confirmButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on confirmButton: ", err)
	}

	// Check for photos icon.
	photosIcon := d.Object(ui.ID(photosIconID))
	if err := photosIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("PhotosIcon doesn't exist: ", err)
	}

}
