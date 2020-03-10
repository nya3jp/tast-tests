// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForSlack = []testutil.TestSuite{
	{"Launch app in Clamshell", launchAppForSlack},
	{"Clamshell: Fullscreen app", testutil.ClamshellFullscreenApp},
	{"Clamshell: Minimise and Restore", testutil.MinimizeRestoreApp},
	{"Clamshell: Resize window", testutil.ClamshellResizeWindow},
	{"Clamshell: Reopen app", testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForSlack = []testutil.TestSuite{
	{"Launch app in Touchview", launchAppForSlack},
	{"Touchview: Minimise and Restore", testutil.MinimizeRestoreApp},
	{"Touchview: Reopen app", testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Slack,
		Desc:         "Functional test for Slack that installs the app also verifies it is logged in and that the main page is open, checks Slack correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				false,
				clamshellTestsForSlack,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedAppCompat(),
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				true,
				touchviewTestsForSlack,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				false,
				clamshellTestsForSlack,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				true,
				touchviewTestsForSlack,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               arc.VMBootedInTabletModeAppCompat(),
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Slack test uses library for opting into the playstore and installing app.
// Checks Slack correctly changes the window states in both clamshell and touchview mode.
func Slack(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.Slack"
		appActivity = "com.Slack.ui.HomeActivity"

		openButtonRegex = "Open|OPEN"
	)

	// Setup Chrome.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()
	s.Log("Enable showing ANRs")
	if err := a.Command(ctx, "settings", "put", "secure", "anr_show_background", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable showing ANRs: ", err)
	}
	s.Log("Enable crash dialog")
	if err := a.Command(ctx, "settings", "put", "secure", "show_first_crash_dialog_dev_option", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable crash dialog: ", err)
	}

	s.Log("Installing app")
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	s.Log("Launch the app")
	// Click on open button.
	openButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches(openButtonRegex))
	must(openButton.WaitForExists(ctx, testutil.LongUITimeout))
	// Open button exist and click.
	must(openButton.Click(ctx))

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
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForSlack verifies Slack is logged in and
// verify Slack reached main activity page of the app.
func launchAppForSlack(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	const (
		signInID         = "com.Slack:id/sign_in_button"
		noneOfTheAboveID = "com.google.android.gms:id/cancel"
		signInManuallyID = "com.Slack:id/sign_in_manually_button"
		workSpaceURLID   = "com.Slack:id/team_url_edit_text"
		workSpace        = "appcompatnetwork"
		nextText         = "Next"
		enterEmailID     = "com.Slack:id/email_edit_text"
		slackEmailID     = "arcplusplusappcompat1@gmail.com"
		slackPassword    = "appcompat"
		enterPasswordID  = "com.Slack:id/password_edit_text"
		homeIconID       = "com.Slack:id/title"
		homeIconText     = "Home"
	)
	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}
	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("signIn Button doesn't exist: ", err)
	} else {
		must(signInButton.Click(ctx))
	}

	// Click on none of the above button.
	noneOfTheButton := d.Object(ui.ID(noneOfTheAboveID))
	if err := noneOfTheButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("noneOfTheButton doesn't exist: ", err)
	} else {
		must(noneOfTheButton.Click(ctx))
	}

	// Click on sign in manually.
	signInManuallyButton := d.Object(ui.ID(signInManuallyID))
	if err := signInManuallyButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("signInManually Button doesn't exist: ", err)
	} else {
		must(signInManuallyButton.Click(ctx))
	}

	// Click on workspace url.
	enterWorkSpaceURL := d.Object(ui.ID(workSpaceURLID))
	if err := enterWorkSpaceURL.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("enterWorkSpaceURL doesn't exist: ", err)
	} else {
		must(enterWorkSpaceURL.Click(ctx))
		must(enterWorkSpaceURL.SetText(ctx, workSpace))
	}

	// Click on next button
	nextButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text("Next"))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("next Button doesn't exist: ", err)
	} else {
		must(nextButton.Click(ctx))
	}

	// Enter email address.
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("enterEmailAddress doesn't exist: ", err)
	} else {
		must(enterEmailAddress.Click(ctx))
		must(enterEmailAddress.SetText(ctx, slackEmailID))
	}

	// Click on next button.
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("next Button doesn't exist: ", err)
	} else {
		must(nextButton.Click(ctx))
	}
	// Enter Password.
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("enterPassword doesn't exist: ", err)
	} else {
		must(enterPassword.Click(ctx))
		must(enterPassword.SetText(ctx, slackPassword))
	}

	// Click on next button.
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("next Button doesn't exist: ", err)
	} else {
		must(nextButton.Click(ctx))
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeIconID), ui.Text(homeIconText))
	must(homeIcon.WaitForExists(ctx, testutil.LongUITimeout))

}
