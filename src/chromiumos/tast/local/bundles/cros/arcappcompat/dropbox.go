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

// clamshellLaunchForDropbox launches Dropbox in clamshell mode.
var clamshellLaunchForDropbox = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForDropbox},
}

// touchviewLaunchForDropbox launches Dropbox in tablet mode.
var touchviewLaunchForDropbox = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForDropbox},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dropbox,
		Desc:         "Functional test for Dropbox that installs the app also verifies it is logged in and that the main page is open, checks Dropbox correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForDropbox,
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
				Tests:      touchviewLaunchForDropbox,
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
				Tests:      clamshellLaunchForDropbox,
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
				Tests:      touchviewLaunchForDropbox,
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
			"arcappcompat.Dropbox.emailid", "arcappcompat.Dropbox.password"},
	})
}

// Dropbox test uses library for opting into the playstore and installing app.
// Checks Dropbox correctly changes the window states in both clamshell and touchview mode.
func Dropbox(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.dropbox.android"
		appActivity = ".activity.DbxMainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForDropbox verifies Dropbox is logged in and
// verify Dropbox reached main activity page of the app.
func launchAppForDropbox(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		confirmText            = "Confirm"
		continueText           = "Continue"
		dismissText            = "Dismiss"
		signInID               = "com.dropbox.android:id/tour_sign_in"
		enterEmailID           = "com.dropbox.android:id/login_email_text_view"
		enterPasswordID        = "com.dropbox.android:id/login_password_text_view"
		submitID               = "com.dropbox.android:id/login_submit"
		cancelDescription      = "Cancel"
		notNowID               = "android:id/autofill_save_no"
		skipText               = "Skip"
		sendSecurityCodeID     = "com.dropbox.android:id/enter_twofactor_code_leadin"
		unLinkDevicesID        = "com.dropbox.android:id/secondary_button"
		unLinkButtonID         = "com.dropbox.android:id/confirmButton"
		selectDevicesClassName = "android.view.ViewGroup"
	)

	var countDevices = 1

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign in button: ", err)
	}

	// Check and click email address.
	DropboxEmailID := s.RequiredVar("arcappcompat.Dropbox.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, DropboxEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	DropboxPassword := s.RequiredVar("arcappcompat.Dropbox.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterPassword doesn't exist: ", err)
	} else if err := enterPassword.SetText(ctx, DropboxPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on submit button.
	submitButton := d.Object(ui.ID(submitID))
	if err := submitButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("submit button doesn't exist: ", err)
	} else if err := submitButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on submit  button: ", err)
	}

	// Check for send security code.
	checkForsendSecurityCode := d.Object(ui.ID(sendSecurityCodeID))
	if err := checkForsendSecurityCode.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("checkForsendSecurityCode doesn't exist")
	} else {
		s.Log("checkForsendSecurityCode does exist")
		return
	}

	// click on notnow button.
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on dismiss button.
	dismissButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(dismissText))
	if err := dismissButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("dismissButton doesn't exist: ", err)
	} else if err := dismissButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on dismissButton: ", err)
	}

	// Click on unlink devices.
	clickOnUnLinkDevices := d.Object(ui.ID(unLinkDevicesID))
	if err := clickOnUnLinkDevices.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnUnLinkDevices doesn't exist: ", err)
	} else if err := clickOnUnLinkDevices.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnUnLinkDevices: ", err)
	}

	// Select devices to unlink until Unlink is enabled.
	// selectDevices := d.Object(ui.ID(selectDevicesID))
	selectDevices := d.Object(ui.ClassName(selectDevicesClassName), ui.Index(countDevices))
	if err := selectDevices.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("selectDevices doesn't exist: ", err)
	} else {
		s.Log("selectDevices does exist")
		for countDevices = 1; countDevices <= 20; countDevices++ {
			selectDevices := d.Object(ui.ClassName(selectDevicesClassName), ui.Index(countDevices))
			if err := selectDevices.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
				s.Log("selectDevices doesn't exist: ", err)
				break
			} else if err := selectDevices.Click(ctx); err != nil {
				s.Log("Failed to click on selectDevices: ", err)
			}
		}
		// Click on unlink button.
		unLinkButton := d.Object(ui.ID(unLinkButtonID), ui.Enabled(true))
		if err := unLinkButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Log("unLinkButton doesn't exist and not enabled yet: ", err)
		} else if err := unLinkButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on unLinkButton: ", err)
		}
	}

	// Click on confirm button.
	confirmButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(confirmText))
	if err := confirmButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("confirmButton doesn't exist: ", err)
	} else if err := confirmButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on confirmButton: ", err)
	}

	// Click on continue button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(continueText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exist: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on cancel button.
	cancelButton := d.Object(ui.Description(cancelDescription))
	if err := cancelButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("cancel button doesn't exist: ", err)
	} else if err := cancelButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on cancel  button: ", err)
	}

	// Click on skip button.
	skipButton := d.Object(ui.Text(skipText))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("skip button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip  button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
