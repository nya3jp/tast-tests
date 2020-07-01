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
var clamshellTestsForSlack = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSlack},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForSlack = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSlack},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Slack,
		Desc:         "Functional test for Slack that installs the app also verifies it is logged in and that the main page is open, checks Slack correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForSlack,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForSlack,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForSlack,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForSlack,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Slack.emailid", "arcappcompat.Slack.password", "arcappcompat.Slack.workspace"},
	})
}

// Slack test uses library for opting into the playstore and installing app.
// Checks Slack correctly changes the window states in both clamshell and touchview mode.
func Slack(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.Slack"
		appActivity = ".ui.HomeActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForSlack verifies Slack is logged in and
// verify Slack reached main activity page of the app.
func launchAppForSlack(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInID         = "com.Slack:id/sign_in_button"
		noneOfTheAboveID = "com.google.android.gms:id/cancel"
		signInManuallyID = "com.Slack:id/sign_in_manually_button"
		workSpaceURLID   = "com.Slack:id/team_url_edit_text"
		nextText         = "Next"
		enterEmailID     = "com.Slack:id/email_edit_text"
		enterPasswordID  = "com.Slack:id/password_edit_text"
		homeIconID       = "com.Slack:id/title"
		homeIconText     = "Home"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SignIn Button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}

	// Click on none of the above button.
	noneOfTheButton := d.Object(ui.ID(noneOfTheAboveID))
	if err := noneOfTheButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("NoneOfTheButton doesn't exist: ", err)
	} else if err := noneOfTheButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on noneOfTheButton: ", err)
	}

	// Click on sign in manually.
	signInManuallyButton := d.Object(ui.ID(signInManuallyID))
	if err := signInManuallyButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SignInManually Button doesn't exist: ", err)
	} else if err := signInManuallyButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInManuallyButton: ", err)
	}

	// Click on workspace url.
	workSpace := s.RequiredVar("arcappcompat.Slack.workspace")
	enterWorkSpaceURL := d.Object(ui.ID(workSpaceURLID))
	if err := enterWorkSpaceURL.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterWorkSpaceURL doesn't exist: ", err)
	} else if err := enterWorkSpaceURL.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterWorkSpaceURL: ", err)
	} else if err := enterWorkSpaceURL.SetText(ctx, workSpace); err != nil {
		s.Fatal("Failed to enterWorkSpaceURL: ", err)
	}

	// Click on next button
	nextButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text("Next"))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("next Button doesn't exist: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	// Enter email address.
	slackEmailID := s.RequiredVar("arcappcompat.Slack.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, slackEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Click on next button.
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Next Button doesn't exist: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}
	// Enter Password.
	slackPassword := s.RequiredVar("arcappcompat.Slack.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, slackPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on next button.
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Next Button doesn't exist: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeIconID), ui.Text(homeIconText))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("HomeIcon doesn't exist: ", err)
	}
}
