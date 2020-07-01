// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForGmail = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForGmail},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForGmail = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForGmail},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gmail,
		Desc:         "Functional test for Gmail that installs the app also verifies it is logged in and that the main page is open, checks Gmail correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForGmail,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForGmail,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForGmail,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForGmail,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Gmail test uses library for opting into the playstore and installing app.
// Checks Gmail correctly changes the window states in both clamshell and touchview mode.
func Gmail(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.gm"
		appActivity = ".ConversationListActivityGmail"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForGmail verifies Gmail is logged in and
// verify Gmail reached main activity page of the app.
func launchAppForGmail(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		composeIconClassName    = "android.widget.ImageButton"
		composeIconDescription  = "Compose"
		textViewClassName       = "android.widget.TextView"
		gotItButtonText         = "GOT IT"
		takeMeToGmailButtonText = "TAKE ME TO GMAIL"
		userNameID              = "com.google.android.gm:id/account_address"
	)

	// Click on Got It button.
	GotItButton := d.Object(ui.ClassName(textViewClassName), ui.Text(gotItButtonText))
	if err := GotItButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("GotIt Button doesn't exist: ", err)
	} else if err := GotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on GotItButton: ", err)
	}

	// Check app is logged in with username.
	userName := d.Object(ui.ID(userNameID))
	if err := userName.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("Username does not exist: ", err)
	}

	// Click on TAKE ME TO GMAIL button.
	takeMeToGmailButton := d.Object(ui.ClassName(textViewClassName), ui.Text(takeMeToGmailButtonText))
	if err := takeMeToGmailButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("TAKE ME TO GMAIL Button doesn't exist: ", err)
	} else if err := takeMeToGmailButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on takeMeToGmailButton: ", err)
	}

	// Check for compose icon in home page.
	composeIcon := d.Object(ui.ClassName(composeIconClassName), ui.DescriptionContains(composeIconDescription))
	if err := composeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("Failed to click on composeIcon: ", err)
	}
}
