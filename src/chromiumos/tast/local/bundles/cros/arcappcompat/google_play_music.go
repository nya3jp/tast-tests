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

// clamshellLaunchForGooglePlayMusic launches GooglePlayMusic in clamshell mode.
var clamshellLaunchForGooglePlayMusic = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForGooglePlayMusic},
}

// touchviewLaunchForGooglePlayMusic launches GooglePlayMusic in tablet mode.
var touchviewLaunchForGooglePlayMusic = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForGooglePlayMusic},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     GooglePlayMusic,
		Desc:     "Functional test for GooglePlayMusic that install, launch the app and check that the main page is open, also checks GooglePlayMusic correctly changes the window state in both clamshell and touchview mode",
		Contacts: []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		// Disabled this test as Google Play Music is no longer available and migrated to Youtube Music.
		//Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForGooglePlayMusic,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForGooglePlayMusic,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForGooglePlayMusic,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForGooglePlayMusic,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GooglePlayMusic test uses library for opting into the playstore and installing app.
// Checks GooglePlayMusic correctly changes the window states in both clamshell and touchview mode.
func GooglePlayMusic(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.music"
		appActivity = "com.android.music.activitymanagement.TopLevelActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForGooglePlayMusic verifies GooglePlayMusic is launched and
// verify GooglePlayMusic reached main activity page of the app.
func launchAppForGooglePlayMusic(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		homeID       = "com.google.android.music:id/navigation_button"
		allowText    = "ALLOW"
		noThanksText = "NO THANKS"
	)
	// Click on allow button.
	clickOnAllowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowText))
	if err := clickOnAllowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnAllowButton doesn't exist: ", err)
	} else if err := clickOnAllowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAllowButton: ", err)
	}

	// Click on noThanks button.
	clickOnNoThanksButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(noThanksText))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("homeIcon doesn't exist: ", err)
	}
}
