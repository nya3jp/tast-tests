// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
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
var clamshellTestsForGoogleCalendar = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForGoogleCalendar},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForGoogleCalendar = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForGoogleCalendar},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleCalendar,
		Desc:         "Functional test for GoogleCalendar that installs the app also verifies it is logged in and that the main page is open, checks GoogleCalendar correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForGoogleCalendar,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForGoogleCalendar,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForGoogleCalendar,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForGoogleCalendar,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GoogleCalendar test uses library for opting into the playstore and installing app.
// Checks GoogleCalendar correctly changes the window states in both clamshell and touchview mode.
func GoogleCalendar(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.calendar"
		appActivity = "com.android.calendar.AllInOneActivity"
	)

	// Step up chrome on Chromebook.
	cr, tconn, a, d := testutil.SetUpDevice(ctx, s, appPkgName, appActivity)
	defer d.Close()

	testSet := s.Param().(testutil.TestParams)
	// Run the different test cases.
	for idx, test := range testSet.Tests {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
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

			defer act.Stop(ctx, tconn)

			// Take screenshot on failure.
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

// launchAppForGoogleCalendar verifies GoogleCalendar is logged in and
// verify GoogleCalendar reached main activity page of the app.
func launchAppForGoogleCalendar(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		addButtonClassName       = "android.widget.ImageButton"
		addButtonDescription     = "Create new event"
		allowButtonText          = "ALLOW"
		androidButtonClassName   = "android.widget.Button"
		gotItButtonText          = "Got it"
		hamburgerIconClassName   = "android.widget.ImageButton"
		hamburgerIconDescription = "Show Calendar List and Settings drawer"
		nextIconID               = "com.google.android.calendar:id/next_arrow_touch"
		openButtonClassName      = "android.widget.Button"
		openButtonRegex          = "Open|OPEN"
		userNameID               = "com.google.android.calendar:id/tile"
	)

	if currentAppPkg := testutil.CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("Entered launchAppForGoogleCalendar and failed to launch the app: ", currentAppPkg)
	}
	s.Log("App launched successfully and entered launchAppForGoogleCalendar")

	// Keep clicking next icon until the got it button exists.
	nextIcon := d.Object(ui.ID(nextIconID))
	gotItButton := d.Object(ui.ClassName(androidButtonClassName), ui.Text(gotItButtonText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := gotItButton.Exists(ctx); err != nil {
			nextIcon.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("GotIt Button doesn't exists: ", err)
	}
	// Click on got it button.
	if err := gotItButton.Exists(ctx); err != nil {
		s.Log("GotIt Button doesn't exist: ", err)
	} else if err := gotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on gotItButton: ", err)
	}

	// Keep clicking allow button until hamburgerIcon exists.
	hamburgerIcon := d.Object(ui.ClassName(hamburgerIconClassName), ui.DescriptionContains(hamburgerIconDescription))
	allowButton := d.Object(ui.ClassName(androidButtonClassName), ui.Text(allowButtonText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := hamburgerIcon.Exists(ctx); err != nil {
			allowButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("hamburgerIcon doesn't exists: ", err)
	}

	// Click on hamburger icon.
	if err := hamburgerIcon.Exists(ctx); err != nil {
		s.Log("hamburgerIcon doesn't exist: ", err)
	} else if err := hamburgerIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on hamburgerIcon: ", err)
	}

	// Check app is logged in with username.
	userName := d.Object(ui.ID(userNameID))
	if err := userName.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("userName doesn't exist: ", err)
	}

	// Click on press back.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		s.Log("Failed to enter KEYCODE_BACK: ", err)
	}

	// Check for add icon in home page.
	addIcon := d.Object(ui.ClassName(addButtonClassName), ui.DescriptionContains(addButtonDescription))
	if err := addIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("addIcon doesn't exist: ", err)
	}
}
