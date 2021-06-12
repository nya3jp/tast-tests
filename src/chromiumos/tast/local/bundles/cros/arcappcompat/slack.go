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

// clamshellLaunchForSlack launches Slack in clamshell mode.
var clamshellLaunchForSlack = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForSlack},
}

// touchviewLaunchForSlack launches Slack in tablet mode.
var touchviewLaunchForSlack = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForSlack},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Slack,
		Desc:         "Functional test for Slack that installs the app also verifies it is logged in and that the main page is open, checks Slack correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForSlack,
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
				Tests:      touchviewLaunchForSlack,
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
				Tests:      clamshellLaunchForSlack,
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
				Tests:      touchviewLaunchForSlack,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Slack.emailid", "arcappcompat.Slack.password", "arcappcompat.Slack.workspace"},
	})
}

// Slack test uses library for opting into the playstore and installing app.
// Checks Slack correctly changes the window states in both clamshell and touchview mode.
func Slack(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.Slack"
		appActivity = "slack.app.ui.HomeActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
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
		notNowID         = "android:id/autofill_save_no"
		neverButtonID    = "com.google.android.gms:id/credential_save_reject"
		enterEmailID     = "com.Slack:id/email_edit_text"
		enterPasswordID  = "com.Slack:id/password"
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

	// Click on never button.
	neverButton := d.Object(ui.ID(neverButtonID))
	if err := neverButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Never Button doesn't exist: ", err)
	} else if err := neverButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on neverButton: ", err)
	}

	// Click on no thanks button.
	clickOnNoThanksButton := d.Object(ui.ID(notNowID))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
