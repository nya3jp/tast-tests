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
var clamshellTestsForConcepts = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForConcepts},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForConcepts = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForConcepts},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Concepts,
		Desc:         "Functional test for Concepts  that installs the app also verifies it is logged in and that the main page is open, checks Gmail correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForConcepts,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForConcepts,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForConcepts,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForConcepts,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Concepts test uses library for opting into the playstore and installing app.
// Checks  Concepts correctly changes the window states in both clamshell and touchview mode.
func Concepts(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.tophatch.concepts"
		appActivity = ".MainActivity"
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

			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForConcepts verifies app is logged in and
// verify app reached main activity page of the app.
func launchAppForConcepts(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		closeButtonClassName = "android.widget.ImageButton"
		closeButtonID        = "com.tophatch.concepts:id/closeButton"
		addButtonID          = "com.tophatch.concepts:id/addButton"
	)

	// Click on close button to launch home page of the app.
	closeButton := d.Object(ui.ClassName(closeButtonClassName), ui.ID(closeButtonID))
	if err := closeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Give Access Button doesn't exists: ", err)
	} else if err := closeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on closeButton: ", err)
	}

	// Check home page is launched.
	addButton := d.Object(ui.ID(addButtonID))
	if err := addButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("AddButton doesn't exists: ", err)
	}
}
