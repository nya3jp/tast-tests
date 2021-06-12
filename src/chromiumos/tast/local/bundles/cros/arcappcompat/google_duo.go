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

// clamshellLaunchForGoogleDuo launches GoogleDuo in clamshell mode.
var clamshellLaunchForGoogleDuo = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForGoogleDuo},
}

// touchviewLaunchForGoogleDuo launches GoogleDuo in tablet mode.
var touchviewLaunchForGoogleDuo = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForGoogleDuo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleDuo,
		Desc:         "Functional test for GoogleDuo that installs the app also verifies it is logged in and that the main page is open, checks GoogleDuo correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_smoke"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForGoogleDuo,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("dru"), hwdep.SkipOnModel("krane")),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForGoogleDuo,
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
				Tests:      clamshellLaunchForGoogleDuo,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("dru"), hwdep.SkipOnModel("krane")),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForGoogleDuo,
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

// GoogleDuo test uses library for opting into the playstore and installing app.
// Checks GoogleDuo correctly changes the window states in both clamshell and touchview mode.
func GoogleDuo(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.apps.tachyon"
		appActivity = ".ui.main.MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForGoogleDuo verifies Google Duo is logged in and
// verify Google Duo reached main activity page of the app.
func launchAppForGoogleDuo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		addPhoneNumberText   = "Add number"
		agreeButtonText      = "Agree"
		allowButtonText      = "ALLOW"
		giveAccessButtonText = "Give access"
		searchContactsText   = "Search contacts or dial"
	)

	// Click on give access button.
	giveAccessButton := d.Object(ui.Text(giveAccessButtonText))
	if err := giveAccessButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Give Access Button doesn't exists: ", err)
	} else if err := giveAccessButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on giveAccessButton: ", err)
	}

	// Keep clicking allow button until add number button exists.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	addPhoneNumberButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(addPhoneNumberText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := addPhoneNumberButton.Exists(ctx); err != nil {
			s.Log("Click on allow button")
			allowButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("addPhoneNumberButton doesn't exists: ", err)
	} else {
		s.Log("addPhoneNumberButton does exists and press back")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Failed to enter KEYCODE_BACK: ", err)
		}
	}

	// Click on agree button.
	agreeButton := d.Object(ui.Text(agreeButtonText))
	if err := agreeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("agreeButton doesn't exists: ", err)
	} else if err := agreeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on agreeButton: ", err)
	}

	// Click on give access button.
	giveAccessButton = d.Object(ui.Text(giveAccessButtonText))
	if err := giveAccessButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Give Access Button doesn't exists: ", err)
	} else if err := giveAccessButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on giveAccessButton: ", err)
	}
	// Keep clicking allow button until add number button exists.
	allowButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	addPhoneNumberButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(addPhoneNumberText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := addPhoneNumberButton.Exists(ctx); err != nil {
			s.Log("Click on allow button")
			allowButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("addPhoneNumberButton doesn't exists: ", err)
	} else {
		s.Log("addPhoneNumberButton does exists and press back")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Failed to enter KEYCODE_BACK: ", err)
		}
	}

	// Check for add your phone number.
	addPhoneNumberButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(addPhoneNumberText))
	if err := addPhoneNumberButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("AddPhoneNumberButton doesn't exists: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		s.Log("Failed to enter KEYCODE_BACK: ", err)
	}

	// Check for search contacts.
	checkForSearchContacts := d.Object(ui.Text(searchContactsText))
	if err := checkForSearchContacts.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("CheckForSearchContacts doesn't exists: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
