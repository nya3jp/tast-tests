// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForDocusign launches Docusign in clamshell mode.
var clamshellLaunchForDocusign = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForDocusign},
}

// touchviewLaunchForDocusign launches Docusign in tablet mode.
var touchviewLaunchForDocusign = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForDocusign},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Docusign,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Docusign that installs the app also verifies it is logged in and that the main page is open, checks Docusign correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForDocusign,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForDocusign,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForDocusign,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForDocusign,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.Docusign.emailid", "arcappcompat.Docusign.password"},
	})
}

// Docusign test uses library for opting into the playstore and installing app.
// Checks Docusign correctly changes the window states in both clamshell and touchview mode.
func Docusign(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.docusign.ink"
		appActivity = ".HomeActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForDocusign verifies Docusign is logged in and
// verify Docusign reached main activity page of the app.
func launchAppForDocusign(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		enterEmailAddressID = "username"
		emailPageClassName  = "android.webkit.WebView"
		continueButtonText  = "CONTINUE"
		notNowID            = "android:id/autofill_save_no"
		passwordID          = "password"
		signinText          = "LOG IN"
		signinButtonText    = "Sign in"
	)

	// Click on signin button.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signinText))
	if err := signInButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("signInButton doesn't exists: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}
	// Check if email address page is in web view.
	// If login page is in web view, the test will skip the login part.
	checkForEmailPage := d.Object(ui.ClassName(emailPageClassName))
	if err := checkForEmailPage.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("checkForEmailPage does not exist: ", err)
	} else {
		s.Log("checkForEmailPage in web view does exist")
		return
	}

	// Enter email address.
	docusignUserName := s.RequiredVar("arcappcompat.Docusign.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterEmailAddress does not exist: ", err)
	}

	if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, docusignUserName); err != nil {
		s.Fatal("Failed to enterUserName: ", err)
	}

	// Click on continue button
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueButtonText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continue Button doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	docusignPassword := s.RequiredVar("arcappcompat.Docusign.password")
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterPassword doesn't exists: ", err)
	}

	if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, docusignPassword); err != nil {
		s.Fatal("Failed to enterUserName: ", err)
	}

	// Click on Sign in button.
	signInButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signinText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Fatal("SignInButton doesn't exists: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}

	// Click on notnow button.
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.Exists(ctx); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
