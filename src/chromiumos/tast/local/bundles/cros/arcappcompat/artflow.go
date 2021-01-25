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
var clamshellTestsForArtflow = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForArtflow},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForArtflow = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForArtflow},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Artflow,
		Desc:         "Functional test for Artflow that install, launch the app and check that the main page is open, also checks Artflow correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForArtflow,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForArtflow,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForArtflow,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForArtflow,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Artflow test uses library for opting into the playstore and installing app.
// Checks Artflow correctly changes the window states in both clamshell and touchview mode.
func Artflow(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.bytestorm.artflow"
		appActivity = ".Editor"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForArtflow verifies Artflow is launched and
// verify Artflow reached main activity page of the app.
func launchAppForArtflow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText             = "ALLOW"
		whileUsingThisAppButtonText = "WHILE USING THE APP"
		selectGmailAccountID        = "com.google.android.gms:id/container"
		homeClassName               = "android.widget.FrameLayout"
	)

	var gmailAccountIndex int

	// Click on allow button.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on allow while using this app button.
	whileUsingThisAppButton := d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))
	if err := whileUsingThisAppButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("whileUsingThisAppButton Button doesn't exists: ", err)
	} else if err := whileUsingThisAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on whileUsingThisAppButton Button: ", err)
	}

	// Click on select gmail account.
	selectSelectGmailAccount := d.Object(ui.ID(selectGmailAccountID), ui.Index(gmailAccountIndex))
	if err := selectSelectGmailAccount.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("selectSelectGmailAccount doesn't exists: ", err)
	} else if err := selectSelectGmailAccount.Click(ctx); err != nil {
		s.Log("Failed to click on selectSelectGmailAccount: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ClassName(homeClassName), ui.PackageName(appPkgName))
	if err := homeIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("homeIcon doesn't exist: ", err)
	}
}
