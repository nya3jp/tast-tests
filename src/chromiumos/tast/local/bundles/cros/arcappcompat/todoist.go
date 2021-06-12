// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

// clamshellLaunchForTodoist launches Todoist in clamshell mode.
var clamshellLaunchForTodoist = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForTodoist},
}

// touchviewLaunchForTodoist launches Todoist in tablet mode.
var touchviewLaunchForTodoist = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForTodoist},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Todoist,
		Desc:         "Functional test for Todoist that installs the app also verifies it is logged in and that the main page is open, checks Todoist correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForTodoist,
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
				Tests:      touchviewLaunchForTodoist,
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
				Tests:      clamshellLaunchForTodoist,
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
				Tests:      touchviewLaunchForTodoist,
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
			"arcappcompat.Todoist.emailid", "arcappcompat.Todoist.password"},
	})
}

// Todoist test uses library for opting into the playstore and installing app.
// Checks Todoist correctly changes the window states in both clamshell and touchview mode.
func Todoist(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.todoist"
		appActivity = ".alias.HomeActivityDefault"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForTodoist verifies Todoist is logged in and
// verify Todoist reached main activity page of the app.
func launchAppForTodoist(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueWithEmailText = "CONTINUE WITH EMAIL"
		noneID                = "com.google.android.gms:id/cancel"
		emailAddressID        = "com.todoist:id/email_exists_input"
		passwordID            = "com.todoist:id/log_in_password"
		logInText             = "LOG IN"
	)

	// Click on continue button.
	continueButton := d.Object(ui.Text(continueWithEmailText))
	if err := continueButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("Continue button doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continue button: ", err)
	}

	// Click on none button.
	noneButton := d.Object(ui.ID(noneID))
	if err := noneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("None button doesn't exist: ", err)
	} else if err := noneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on none button: ", err)
	}

	// Click on email address.
	TodoistEmailID := s.RequiredVar("arcappcompat.Todoist.emailid")
	emailAddress := d.Object(ui.ID(emailAddressID))
	if err := emailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	} else if err := emailAddress.SetText(ctx, TodoistEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Click on continueButton until enterPassword exist.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := continueButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("Continue button doesn't exist: ", err)
	} else if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := enterPassword.Exists(ctx); err != nil {
			continueButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("enterPassword doesn't exist: ", err)
	}

	// Enter password.
	TodoistPassword := s.RequiredVar("arcappcompat.Todoist.password")
	enterPassword = d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, TodoistPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on login button.
	logInButton := d.Object(ui.Text(logInText))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Log in button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on log in button: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
