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
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForGoogleCalendar launches GoogleCalendar in clamshell mode.
var clamshellLaunchForGoogleCalendar = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForGoogleCalendar},
}

// touchviewLaunchForGoogleCalendar launches GoogleCalendar in tablet mode.
var touchviewLaunchForGoogleCalendar = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForGoogleCalendar},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleCalendar,
		Desc:         "Functional test for GoogleCalendar that installs the app also verifies it is logged in and that the main page is open, checks GoogleCalendar correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForGoogleCalendar,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForGoogleCalendar,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForGoogleCalendar,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForGoogleCalendar,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GoogleCalendar test uses library for opting into the playstore and installing app.
// Checks GoogleCalendar correctly changes the window states in both clamshell and touchview mode.
func GoogleCalendar(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.calendar"
		appActivity = "com.android.calendar.AllInOneActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForGoogleCalendar verifies GoogleCalendar is logged in and
// verify GoogleCalendar reached main activity page of the app.
func launchAppForGoogleCalendar(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		allowButtonText          = "ALLOW"
		androidButtonClassName   = "android.widget.Button"
		gotItButtonText          = "Got it"
		hamburgerIconClassName   = "android.widget.ImageButton"
		hamburgerIconDescription = "Show Calendar List and Settings drawer"
		nextIconID               = "com.google.android.calendar:id/next_arrow"
		openButtonClassName      = "android.widget.Button"
		openButtonRegex          = "Open|OPEN"
		userNameID               = "com.google.android.calendar:id/tile"
	)

	// Keep clicking next icon until the got it button exists.
	nextIcon := d.Object(ui.ID(nextIconID))
	if err := nextIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("nextIcon doesn't exists: ", err)
	}

	gotItButton := d.Object(ui.ClassName(androidButtonClassName), ui.TextMatches("(?i)"+gotItButtonText))
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
	allowButton := d.Object(ui.ClassName(androidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
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
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		s.Log("Failed to enter KEYCODE_BACK: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
