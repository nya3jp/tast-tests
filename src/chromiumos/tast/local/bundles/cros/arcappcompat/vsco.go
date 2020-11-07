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
)

// ClamshellTests are placed here.
var clamshellTestsForVSCO = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForVsco},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForVSCO = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForVsco},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Vsco,
		Desc:         "Functional test for Vsco that install, launch the app and check that the main page is open, also checks Vsco correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForVSCO,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForVSCO,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForVSCO,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForVSCO,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Vsco test uses library for opting into the playstore and installing app.
// Checks Vsco correctly changes the window states in both clamshell and touchview mode.
func Vsco(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.vsco.cam"
		appActivity = ".navigation.LithiumActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForVsco verifies Vsco is launched and
// verify Vsco reached main activity page of the app.
func launchAppForVsco(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText                = "ALLOW"
		continueText                   = "Continue"
		continueClassName              = "android.widget.TextView"
		continueID                     = "com.vsco.cam:id/create_username_button"
		closeID                        = "com.vsco.cam:id/close_button"
		closeButtonID                  = "com.vsco.cam:id/header_left_button"
		emailAddressID                 = "com.google.android.gms:id/container"
		loginWithGoogleButtonClassName = "android.widget.Button"
		loginWithGoogleButtonText      = "Continue with Google"
		homeID                         = "com.vsco.cam:id/import_fab"
	)

	loginWithGoogleButton := d.Object(ui.ClassName(loginWithGoogleButtonClassName), ui.TextMatches("(?i)"+loginWithGoogleButtonText))
	emailAddress := d.Object(ui.ID(emailAddressID))
	// Click on login with Google Button until EmailAddress exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := emailAddress.Exists(ctx); err != nil {
			loginWithGoogleButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("emailAddress doesn't exist: ", err)
	}
	// Click on email address.
	if err := emailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	}

	// Click on continue button.
	continueButton := d.Object(ui.ClassName(continueClassName), ui.TextMatches("(?i)"+continueText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on close button.
	closeButton := d.Object(ui.ID(closeID))
	if err := closeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("closeButton doesn't exists: ", err)
	} else if err := closeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on closeButton: ", err)
	}

	// Click on continue button.
	continueButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// click on allow button until clode button exists.
	closeButton = d.Object(ui.ID(closeButtonID))
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := closeButton.Exists(ctx); err != nil {
			allowButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("closeButton doesn't exists: ", err)
	} else if err := closeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on closeButton: ", err)
	}

	// Click on home icon.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("homeIcon doesn't exists: ", err)
	}

}
