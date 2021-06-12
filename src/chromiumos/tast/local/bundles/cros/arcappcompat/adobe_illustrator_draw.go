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

// clamshellLaunchForAdobeIllustratorDraw launches AdobeIllustratorDraw in clamshell mode.
var clamshellLaunchForAdobeIllustratorDraw = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForAdobeIllustratorDraw},
}

// touchviewLaunchForAdobeIllustratorDraw launches AdobeIllustratorDraw in tablet mode.
var touchviewLaunchForAdobeIllustratorDraw = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForAdobeIllustratorDraw},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdobeIllustratorDraw,
		Desc:         "Functional test for AdobeIllustratorDraw that installs the app also verifies it is logged in and that the main page is open, checks AdobeIllustratorDraw correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("eve")),
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForAdobeIllustratorDraw,
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
				Tests:      touchviewLaunchForAdobeIllustratorDraw,
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
				Tests:      clamshellLaunchForAdobeIllustratorDraw,
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
				Tests:      touchviewLaunchForAdobeIllustratorDraw,
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

// AdobeIllustratorDraw test uses library for opting into the playstore and installing app.
// Checks AdobeIllustratorDraw correctly changes the window states in both clamshell and touchview mode.
func AdobeIllustratorDraw(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.adobe.creativeapps.draw"
		appActivity = ".activity.SplashActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForAdobeIllustratorDraw verify app is logged in and
// verify app reached main activity page of the app.
func launchAppForAdobeIllustratorDraw(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		addProjectIconID     = "com.adobe.creativeapps.draw:id/add_project_btn"
		continueButtonText   = "Continue"
		checkBoxID           = "consent"
		selectGmailAccountID = "com.google.android.gms:id/container"
		signInWithAGoogleID  = "com.adobe.creativeapps.draw:id/tvSignInButtonWithGoogle"
	)

	// Check for sign in button.
	signInButton := d.Object(ui.ID(signInWithAGoogleID))
	if err := signInButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("signInButton doesn't exists: ", err)
	}

	// Click on signInButton until selectGmailAccount exist.
	selectGmailAccount := d.Object(ui.ID(selectGmailAccountID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := selectGmailAccount.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("selectGmailAccount doesn't exist: ", err)
	}

	// For selecting Gmail account
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
	}

	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	} else {
		s.Log("Entered KEYCODE_ENTER")
	}

	// Click on agree check box.
	clickOnCheckBox := d.Object(ui.ID(checkBoxID))
	if err := clickOnCheckBox.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnCheckBox doesn't exists: ", err)
	} else if err := clickOnCheckBox.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnCheckBox: ", err)
	}

	// Click on continue button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(continueButtonText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for homePageVerifier.
	homePageVerifier := d.Object(ui.ID(addProjectIconID))
	if err := homePageVerifier.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Fatal("homePageVerifier doesn't exists: ", err)
	}
}
