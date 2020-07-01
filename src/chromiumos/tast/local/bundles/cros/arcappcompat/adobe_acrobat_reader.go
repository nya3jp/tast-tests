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
var clamshellTestsForAdobeAcrobatReader = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForAdobeAcrobatReader},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForAdobeAcrobatReader = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForAdobeAcrobatReader},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdobeAcrobatReader,
		Desc:         "Functional test for AdobeAcrobatReader that installs the app also verifies it is logged in and that the main page is open, checks AdobeAcrobatReader correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForAdobeAcrobatReader,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForAdobeAcrobatReader,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForAdobeAcrobatReader,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForAdobeAcrobatReader,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// AdobeAcrobatReader test uses library for opting into the playstore and installing app.
// Checks AdobeAcrobatReader correctly changes the window states in both clamshell and touchview mode.
func AdobeAcrobatReader(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.adobe.reader"
		appActivity = ".AdobeReader"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForAdobeAcrobatReader verify app is logged in and
// verify app reached main activity page of the app.
func launchAppForAdobeAcrobatReader(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		continueButtonText  = "Continue"
		continueButtonID    = "com.adobe.reader:id/continue_button"
		closeClassName      = "android.widget.ImageButton"
		closeDes            = "Close tour"
		homeIconClassName   = "android.view.ViewGroup"
		homeIconDescription = "Home"
		signInButtonText    = "Sign in with Google"
		userButtonClassName = "android.widget.TextView"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ClassName(userButtonClassName), ui.Text(signInButtonText))
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

	// Click on continue button.
	continueButton := d.Object(ui.ID(continueButtonID), ui.Text(continueButtonText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on close button.
	closeButton := d.Object(ui.ClassName(closeClassName), ui.Description(closeDes))
	if err := closeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("closeButton doesn't exists: ", err)
	} else if err := closeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on closeButton: ", err)
	}

	// Check for home icon in home page.
	homeIcon := d.Object(ui.ClassName(homeIconClassName), ui.DescriptionContains(homeIconDescription))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("homeIcon doesn't exists: ", err)
	}
}
