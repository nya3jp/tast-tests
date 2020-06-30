// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForAdobeIllustratorDraw = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForAdobeIllustratorDraw},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForAdobeIllustratorDraw = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForAdobeIllustratorDraw},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdobeIllustratorDraw,
		Desc:         "Functional test for AdobeIllustratorDraw that installs the app also verifies it is logged in and that the main page is open, checks AdobeIllustratorDraw correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForAdobeIllustratorDraw,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForAdobeIllustratorDraw,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForAdobeIllustratorDraw,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForAdobeIllustratorDraw,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// AdobeIllustratorDraw test uses library for opting into the playstore and installing app.
// Checks AdobeIllustratorDraw correctly changes the window states in both clamshell and touchview mode.
func AdobeIllustratorDraw(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.adobe.creativeapps.draw"
		appActivity = ".activity.SplashActivity"
	)

	// Step up chrome on Chromebook.
	cr, tconn, a, d := testutil.SetUpDevice(ctx, s, appPkgName, appActivity)
	defer d.Close()

	// Ensure app launches before test cases.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app before test cases: ", err)
	}
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app before test cases: ", err)
	}

	testSet := s.Param().(testutil.TestParams)
	// Run the different test cases.
	for idx, test := range testSet.Tests {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			// Launch the app.
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			defer act.Stop(ctx, tconn)

			// Take screenshot on failure.
			defer func() {
				if s.HasError() {
					filename := fmt.Sprintf("screenshot-arcappcompat-failed-test-%d.png", idx)
					path := filepath.Join(s.OutDir(), filename)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
				}
			}()

			testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
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

	if currentAppPkg := testutil.CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("Entered launchAppForAdobeIllustratorDraw and failed to launch the app: ", currentAppPkg)
	}
	s.Log("App launched successfully and entered launchAppForAdobeIllustratorDraw")

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInWithAGoogleID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("signInButton doesn't exists: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
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

	// Check for add project icon in home page.
	addProjectIcon := d.Object(ui.ID(addProjectIconID))
	if err := addProjectIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("addProjectIcon doesn't exists: ", err)
	}
}
