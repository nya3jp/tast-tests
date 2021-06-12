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

// clamshellLaunchForArtflow launches Artflow in clamshell mode.
var clamshellLaunchForArtflow = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForArtflow},
}

// touchviewLaunchForArtflow launches Artflow in tablet mode.
var touchviewLaunchForArtflow = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForArtflow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Artflow,
		Desc:         "Functional test for Artflow that install, launch the app and check that the main page is open, also checks Artflow correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForArtflow,
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
				Tests:      touchviewLaunchForArtflow,
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
				Tests:      clamshellLaunchForArtflow,
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
				Tests:      touchviewLaunchForArtflow,
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

// Artflow test uses library for opting into the playstore and installing app.
// Checks Artflow correctly changes the window states in both clamshell and touchview mode.
func Artflow(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.bytestorm.artflow"
		appActivity = ".Editor"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForArtflow verifies Artflow is launched and
// verify Artflow reached main activity page of the app.
func launchAppForArtflow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText             = "ALLOW"
		whileUsingThisAppButtonText = "WHILE USING THE APP"
		selectGmailAccountID        = "com.google.android.gms:id/container"
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

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
