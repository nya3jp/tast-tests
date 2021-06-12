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

// clamshellLaunchForGmail launches Gmail in clamshell mode.
var clamshellLaunchForGmail = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForGmail},
}

// touchviewLaunchForGmail launches Gmail in tablet mode.
var touchviewLaunchForGmail = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForGmail},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gmail,
		Desc:         "Functional test for Gmail that installs the app also verifies it is logged in and that the main page is open, checks Gmail correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_smoke"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForGmail,
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
				Tests:      touchviewLaunchForGmail,
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
				Tests:      clamshellLaunchForGmail,
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
				Tests:      touchviewLaunchForGmail,
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

// Gmail test uses library for opting into the playstore and installing app.
// Checks Gmail correctly changes the window states in both clamshell and touchview mode.
func Gmail(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.gm"
		appActivity = ".ConversationListActivityGmail"
	)

	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForGmail verifies Gmail is logged in and
// verify Gmail reached main activity page of the app.
func launchAppForGmail(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		textViewClassName       = "android.widget.TextView"
		gotItButtonText         = "GOT IT"
		takeMeToGmailButtonText = "TAKE ME TO GMAIL"
		userNameID              = "com.google.android.gm:id/account_address"
	)

	// Click on got It button.
	gotItButton := d.Object(ui.ClassName(textViewClassName), ui.Text(gotItButtonText))
	if err := gotItButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("gotItButton doesn't exist: ", err)
	} else if err := gotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on gotItButton: ", err)
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

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
