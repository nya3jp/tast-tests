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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForPandora launches Pandora in clamshell mode.
var clamshellLaunchForPandora = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForPandora},
}

// touchviewLaunchForPandora launches Pandora in tablet mode.
var touchviewLaunchForPandora = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForPandora},
}

// clamshellAppSpecificTestsForPandora are placed here.
var clamshellAppSpecificTestsForPandora = []testutil.TestCase{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForPandora are placed here.
var touchviewAppSpecificTestsForPandora = []testutil.TestCase{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pandora,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Pandora that installs the app also verifies it is logged in and that the main page is open, checks Pandora correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForPandora,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForPandora,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForPandora,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForPandora,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForPandora,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForPandora,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForPandora,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForPandora,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.Pandora.emailid", "arcappcompat.Pandora.password"},
	})
}

// Pandora test uses library for opting into the playstore and installing app.
// Checks Pandora correctly changes the window states in both clamshell and touchview mode.
func Pandora(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.pandora.android"
		appActivity = ".LauncherActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForPandora verifies Pandora is logged in and
// verify Pandora reached main activity page of the app.
func launchAppForPandora(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInID           = "com.pandora.android:id/welcome_log_in_button"
		emailText          = "Email"
		passwordText       = "Password"
		logInText          = "Log In"
		noneOfTheAboveText = "NONE OF THE ABOVE"
		neverButtonID      = "com.google.android.gms:id/credential_save_reject"
		notNowID           = "android:id/autofill_save_no"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("Sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Sign in button: ", err)
	}

	// Click on none of the above button to skip login using gmail account.
	noneOfTheButton := d.Object(ui.TextMatches("(?i)" + noneOfTheAboveText))
	if err := noneOfTheButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("NoneOfTheButton doesn't exist: ", err)
	} else if err := noneOfTheButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on noneOfTheButton: ", err)
	}

	// Check and click email address.
	pandoraEmailID := s.RequiredVar("arcappcompat.Pandora.emailid")
	enterEmailAddress := d.Object(ui.Text(emailText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	if err := noneOfTheButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("NoneOfTheButton doesn't exist: ", err)
	} else if err := noneOfTheButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on noneOfTheButton: ", err)
	}

	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.SetText(ctx, pandoraEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	pandoraPassword := s.RequiredVar("arcappcompat.Pandora.password")
	enterPassword := d.Object(ui.Text(passwordText))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, pandoraPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	deviceMode := "clamshell"
	if tabletModeEnabled {
		deviceMode = "tablet"
		// Press back to make login button visible.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Failed to enter KEYCODE_BACK: ", err)
		} else {
			s.Log("Entered KEYCODE_BACK")
		}
	}
	s.Logf("device %v mode", deviceMode)

	// Click on log in button
	logInButton := d.Object(ui.Text(logInText))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogIn button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on LogIn button: ", err)
	}

	// Click on never button.
	neverButton := d.Object(ui.ID(neverButtonID))
	if err := neverButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Never Button doesn't exist: ", err)
	} else if err := neverButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on neverButton: ", err)
	}

	// Click on no thanks button.
	clickOnNoThanksButton := d.Object(ui.ID(notNowID))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
