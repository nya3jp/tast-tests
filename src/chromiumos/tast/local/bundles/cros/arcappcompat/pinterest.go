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

// clamshellLaunchForPinterest launches Pinterest in clamshell mode.
var clamshellLaunchForPinterest = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForPinterest},
}

// touchviewLaunchForPinterest launches Pinterest in tablet mode.
var touchviewLaunchForPinterest = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForPinterest},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     Pinterest,
		Desc:     "Functional test for Pinterest that installs the app also verifies it is logged in and that the main page is open, checks Pinterest correctly changes the window state in both clamshell and touchview mode",
		Contacts: []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		// Disabled this test as Pinterest android app is not available anymore and it is migrated to PWA app.
		//Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForPinterest,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForPinterest,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForPinterest,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForPinterest,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Pinterest test uses library for opting into the playstore and installing app.
// Checks Pinterest correctly changes the window states in both clamshell and touchview mode.
func Pinterest(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.pinterest"
		appActivity = ".activity.PinterestActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForPinterest verifies Pinterest is logged in and
// verify Pinterest reached main activity page of the app.
func launchAppForPinterest(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText                = "ALLOW"
		emailAddressID                 = "com.google.android.gms:id/account_name"
		loginWithGoogleButtonClassName = "android.widget.Button"
		loginWithGoogleButtonText      = "Continue with Google"
		profileIconID                  = "com.pinterest:id/profile_menu_view"
		turnOnLocationText             = "Turn on location services"
		nextText                       = "NEXT"
		whileUsingThisAppButtonText    = "WHILE USING THE APP"
	)

	loginWithGoogleButton := d.Object(ui.ClassName(loginWithGoogleButtonClassName), ui.Text(loginWithGoogleButtonText))
	emailAddress := d.Object(ui.ID(emailAddressID))
	// Click on login with Google Button until EmailAddress exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := emailAddress.Exists(ctx); err != nil {
			loginWithGoogleButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("emailAddress doesn't exist: ", err)
	}
	// Click on email address.
	if err := emailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	}

	// Click on next button.
	nextButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(nextText))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("nextButton doesn't exist: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	// Click on turn on location button.
	turnOnLocationButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(turnOnLocationText))
	if err := turnOnLocationButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("turnOnLocationButton doesn't exist: ", err)
	} else if err := turnOnLocationButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on turnOnLocationButton: ", err)
	}

	// Click on allow while using this app button to access files.
	clickOnWhileUsingThisAppButton := d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))
	if err := clickOnWhileUsingThisAppButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnWhileUsingThisApp Button doesn't exists: ", err)
	} else if err := clickOnWhileUsingThisAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnWhileUsingThisApp Button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
