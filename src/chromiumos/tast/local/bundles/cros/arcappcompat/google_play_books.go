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

// ClamshellTests are placed here.
var clamshellTestsForGooglePlayBooks = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForGooglePlayBooks},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForGooglePlayBooks = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForGooglePlayBooks},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GooglePlayBooks,
		Desc:         "Functional test for GooglePlayBooks that install, launch the app and check that the main page is open, also checks GooglePlayBooks correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForGooglePlayBooks,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForGooglePlayBooks,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForGooglePlayBooks,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForGooglePlayBooks,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GooglePlayBooks test uses library for opting into the playstore and installing app.
// Checks GooglePlayBooks correctly changes the window states in both clamshell and touchview mode.
func GooglePlayBooks(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.apps.books"
		appActivity = ".app.BooksActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForGooglePlayBooks verifies GooglePlayBooks is launched and
// verify GooglePlayBooks reached main activity page of the app.
func launchAppForGooglePlayBooks(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		okText          = "OK"
		getStartedText  = "Get started"
		selectAccountID = "android:id/text1"
		homeID          = "com.google.android.apps.books:id/bottom_home"
	)
	// Click on select account.
	clickOnSelectAccount := d.Object(ui.ID(selectAccountID))
	if err := clickOnSelectAccount.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnSelectAccount doesn't exist: ", err)
	} else if err := clickOnSelectAccount.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnSelectAccount: ", err)
	}
	// Click on ok button.
	clickOnOkButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(okText))
	if err := clickOnOkButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnOkButton doesn't exist: ", err)
	} else if err := clickOnOkButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnOkButton: ", err)
	}

	// Click on get started button.
	clickOnGetStartedButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(getStartedText))
	if err := clickOnGetStartedButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnGetStartedButton doesn't exist: ", err)
	} else if err := clickOnGetStartedButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnGetStartedButton: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("homeIcon doesn't exist: ", err)
	}
}
