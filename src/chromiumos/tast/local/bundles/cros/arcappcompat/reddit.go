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

// ClamshellTests are placed here.
var clamshellTestsForReddit = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForReddit},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForReddit = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForReddit},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Reddit,
		Desc:         "Functional test for Reddit that installs the app also verifies it is logged in and that the main page is open, checks Reddit correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForReddit,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForReddit,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("careena"), hwdep.SkipOnModel("kasumi"), hwdep.SkipOnModel("treeya"),
				hwdep.SkipOnModel("bluebird"), hwdep.SkipOnModel("sarien"), hwdep.SkipOnModel("elemi"), hwdep.SkipOnModel("berknip"),
				hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("aleena"), hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("grunt"),
				hwdep.SkipOnModel("liara"), hwdep.SkipOnModel("nuwani"), hwdep.SkipOnModel("kindred"), hwdep.SkipOnModel("dratini"),
				hwdep.SkipOnModel("apel"), hwdep.SkipOnModel("blooglet"), hwdep.SkipOnModel("blorb"), hwdep.SkipOnModel("bobba"),
				hwdep.SkipOnModel("casta"), hwdep.SkipOnModel("dorp"), hwdep.SkipOnModel("droid"), hwdep.SkipOnModel("fleex"),
				hwdep.SkipOnModel("foob"), hwdep.SkipOnModel("garfour"), hwdep.SkipOnModel("garg"), hwdep.SkipOnModel("laser14"),
				hwdep.SkipOnModel("lick"), hwdep.SkipOnModel("mimrock"), hwdep.SkipOnModel("nospike"), hwdep.SkipOnModel("orbatrix"),
				hwdep.SkipOnModel("phaser"), hwdep.SkipOnModel("sparky")),
			Pre: pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForReddit,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForReddit,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("careena"), hwdep.SkipOnModel("kasumi"), hwdep.SkipOnModel("treeya"),
				hwdep.SkipOnModel("bluebird"), hwdep.SkipOnModel("sarien"), hwdep.SkipOnModel("elemi"), hwdep.SkipOnModel("berknip"),
				hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("aleena"), hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("grunt"),
				hwdep.SkipOnModel("liara"), hwdep.SkipOnModel("nuwani"), hwdep.SkipOnModel("kindred"), hwdep.SkipOnModel("dratini"),
				hwdep.SkipOnModel("apel"), hwdep.SkipOnModel("blooglet"), hwdep.SkipOnModel("blorb"), hwdep.SkipOnModel("bobba"),
				hwdep.SkipOnModel("casta"), hwdep.SkipOnModel("dorp"), hwdep.SkipOnModel("droid"), hwdep.SkipOnModel("fleex"),
				hwdep.SkipOnModel("foob"), hwdep.SkipOnModel("garfour"), hwdep.SkipOnModel("garg"), hwdep.SkipOnModel("laser14"),
				hwdep.SkipOnModel("lick"), hwdep.SkipOnModel("mimrock"), hwdep.SkipOnModel("nospike"), hwdep.SkipOnModel("orbatrix"),
				hwdep.SkipOnModel("phaser"), hwdep.SkipOnModel("sparky")),
			Pre: pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Reddit test uses library for opting into the playstore and installing app.
// Checks Reddit correctly changes the window states in both clamshell and touchview mode.
func Reddit(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.reddit.frontpage"
		appActivity = "launcher.default"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForReddit verifies Reddit is logged in and
// verify Reddit reached main activity page of the app.
func launchAppForReddit(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText             = "ALLOW"
		continueID                  = "com.google.android.gms:id/continue_button"
		continueText                = "Continue"
		skipButtonText              = "Skip"
		whileUsingThisAppButtonText = "WHILE USING THE APP"
	)
	// Click on continue button to sign in using gmail account.
	continueButton := d.Object(ui.TextStartsWith("(?i)" + continueText))
	if err := continueButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("continueButton doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on skip button.
	skipButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+skipButtonText))
	if err := skipButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("skipButton doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}

	// Click on continue button to sign in using gmail account.
	continueButton = d.Object(ui.ID(continueID))
	if err := continueButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("continueButton doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on continue button to sign back in Reddit using gmail account.
	continueButton = d.Object(ui.ID(continueID))
	if err := continueButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("continueButton doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on skip button.
	skipButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+skipButtonText))
	if err := skipButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("skipButton doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on allow while using this app button to record audio.
	clickOnWhileUsingThisAppButton := d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))
	if err := clickOnWhileUsingThisAppButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnWhileUsingThisApp Button doesn't exists: ", err)
	} else if err := clickOnWhileUsingThisAppButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnWhileUsingThisApp Button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
