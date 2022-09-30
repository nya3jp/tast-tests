// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForAmazonPrimeVideo launches AmazonPrimeVideo in clamshell mode.
var clamshellLaunchForAmazonPrimeVideo = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForAmazonPrimeVideo, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForAmazonPrimeVideo launches AmazonPrimeVideo in tablet mode.
var touchviewLaunchForAmazonPrimeVideo = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForAmazonPrimeVideo, Timeout: testutil.LaunchTestCaseTimeout},
}

// clamshellAppSpecificTestsForAmazonPrimeVideo are placed here.
var clamshellAppSpecificTestsForAmazonPrimeVideo = []testutil.TestCase{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Clamshell: Signout app", Fn: signOutAmazonPrimeVideo, Timeout: testutil.SignoutTestCaseTimeout},
}

// touchviewAppSpecificTestsForAmazonPrimeVideo are placed here.
var touchviewAppSpecificTestsForAmazonPrimeVideo = []testutil.TestCase{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Touchview: Signout app", Fn: signOutAmazonPrimeVideo, Timeout: testutil.SignoutTestCaseTimeout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AmazonPrimeVideo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for AmazonPrimeVideo that installs the app also verifies it is logged in and that the main page is open, checks AmazonPrimeVideo correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "clamshell_mode_default",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForAmazonPrimeVideo,
					CommonTests:      testutil.ClamshellCommonTests,
					AppSpecificTests: clamshellAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_default"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			}, {
				Name: "tablet_mode_default",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForAmazonPrimeVideo,
					CommonTests:      testutil.TouchviewCommonTests,
					AppSpecificTests: touchviewAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_default"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}, {
				Name: "vm_clamshell_mode_default",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForAmazonPrimeVideo,
					CommonTests:      testutil.ClamshellCommonTests,
					AppSpecificTests: clamshellAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_default"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			}, {
				Name: "vm_tablet_mode_default",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForAmazonPrimeVideo,
					CommonTests:      testutil.TouchviewCommonTests,
					AppSpecificTests: touchviewAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_default"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}, {
				Name: "clamshell_mode_smoke",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForAmazonPrimeVideo,
					CommonTests:      testutil.ClamshellSmokeTests,
					AppSpecificTests: clamshellAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_smoke"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			}, {
				Name: "tablet_mode_smoke",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForAmazonPrimeVideo,
					CommonTests:      testutil.TouchviewSmokeTests,
					AppSpecificTests: touchviewAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_smoke"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}, {
				Name: "vm_clamshell_mode_smoke",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForAmazonPrimeVideo,
					CommonTests:      testutil.ClamshellSmokeTests,
					AppSpecificTests: clamshellAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_smoke"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			}, {
				Name: "vm_tablet_mode_smoke",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForAmazonPrimeVideo,
					CommonTests:      testutil.TouchviewSmokeTests,
					AppSpecificTests: touchviewAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_smoke"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}, {
				Name: "clamshell_mode_top_apps",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForAmazonPrimeVideo,
					TopAppTests:      testutil.ClamshellTopAppTests,
					AppSpecificTests: clamshellAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_top_apps"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			}, {
				Name: "tablet_mode_top_apps",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForAmazonPrimeVideo,
					TopAppTests:      testutil.TouchviewTopAppTests,
					AppSpecificTests: touchviewAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_top_apps"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}, {
				Name: "vm_clamshell_mode_top_apps",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForAmazonPrimeVideo,
					TopAppTests:      testutil.ClamshellTopAppTests,
					AppSpecificTests: clamshellAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_top_apps"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			}, {
				Name: "vm_tablet_mode_top_apps",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForAmazonPrimeVideo,
					TopAppTests:      testutil.TouchviewTopAppTests,
					AppSpecificTests: touchviewAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_top_apps"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}, {
				Name: "clamshell_mode_release",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForAmazonPrimeVideo,
					ReleaseTests:     testutil.ClamshellReleaseTests,
					AppSpecificTests: clamshellAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_release"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			}, {
				Name: "tablet_mode_release",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForAmazonPrimeVideo,
					ReleaseTests:     testutil.TouchviewReleaseTests,
					AppSpecificTests: touchviewAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_release"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}, {
				Name: "vm_clamshell_mode_release",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForAmazonPrimeVideo,
					ReleaseTests:     testutil.ClamshellReleaseTests,
					AppSpecificTests: clamshellAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_release"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			}, {
				Name: "vm_tablet_mode_release",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForAmazonPrimeVideo,
					ReleaseTests:     testutil.TouchviewReleaseTests,
					AppSpecificTests: touchviewAppSpecificTestsForAmazonPrimeVideo,
				},
				ExtraAttr:         []string{"appcompat_release"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}},
		Timeout: 30 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.AmazonPrimeVideo.username", "arcappcompat.AmazonPrimeVideo.password"},
	})
}

// AmazonPrimeVideo test uses library for opting into the playstore and installing app.
// Checks AmazonPrimeVideo correctly changes the window states in both clamshell and touchview mode.
func AmazonPrimeVideo(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.amazon.avod.thirdpartyclient"
		appActivity = ".LauncherActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForAmazonPrimeVideo verifies AmazonPrimeVideo is logged in and
// verify AmazonPrimeVideo reached main activity page of the app.
func launchAppForAmazonPrimeVideo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText      = "ALLOW"
		enterEmailAddressID  = "ap_email"
		letsGoButtonID       = "com.amazon.avod.thirdpartyclient:id/cs_intro_cx_modal_button"
		passwordClassName    = "android.widget.EditText"
		passwordID           = "ap_password"
		passwordText         = "Amazon password"
		signInText           = "Sign-In"
		sendOTPText          = "Send OTP"
		notNowID             = "android:id/autofill_save_no"
		neverButtonID        = "com.google.android.gms:id/credential_save_reject"
		importantMessageText = "Important"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterEmailAddress does not exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}
	// Click on emailid text field until the emailid text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if emailIDFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
			return errors.New("email text field not focused yet")
		} else if !emailIDFocused {
			enterEmailAddress.Click(ctx)
			return errors.New("email text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus EmailId: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.AmazonPrimeVideo.username")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterPassword does not exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	// Click on password text field until the password text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
			return errors.New("password text field not focused yet")
		} else if !pwdFocused {
			enterPassword.Click(ctx)
			return errors.New("password text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.AmazonPrimeVideo.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on not now button until sign in page is visible.
	signInPage := d.Object(ui.PackageName(appPkgName))
	notNowButton := d.Object(ui.ID(notNowID))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, notNowButton, signInPage)

	// Press enter button to click on sign in button.
	d.WaitForIdle(ctx, testutil.ShortUITimeout)
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	} else {
		s.Log("Entered KEYCODE_ENTER")
	}
	// Click on never button until home page is visible
	neverButton := d.Object(ui.ID(neverButtonID))
	letsGoButton := d.Object(ui.ID(letsGoButtonID))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, neverButton, letsGoButton)

	// Click on lets go button.
	if err := letsGoButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LetsGo Button doesn't exists: ", err)
	} else if err := letsGoButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on LetsGo Button: ", err)
	}
	// Check for captcha and OTP.
	checkForCaptcha := d.Object(ui.TextStartsWith(importantMessageText))
	sendOTPButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(sendOTPText))

	if err := checkForCaptcha.WaitForExists(ctx, testutil.DefaultUITimeout); err == nil {
		s.Log("checkForCaptcha does exists")
		return
	}
	if err := sendOTPButton.WaitForExists(ctx, testutil.DefaultUITimeout); err == nil {
		s.Log("Send OTP Button does exist")
		return
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}

// signOutAmazonPrimeVideo verifies app is signed out.
func signOutAmazonPrimeVideo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		enterEmailAddressID           = "ap_email"
		layoutClassName               = "android.widget.FrameLayout"
		profileIconID                 = "com.amazon.avod.thirdpartyclient:id/header_bar_profile_avatar"
		profileIconDes                = "Profiles and My Stuff"
		settingsIconClassName         = "android.widget.ImageView"
		settingsIconDescription       = "Settings"
		selectSignedInOptionClassName = "android.widget.TextView"
		selectSignedInOptionText      = "Signed in as arcapps"
		signOutText                   = "Sign out"
	)

	// Click on profile icon.
	profileIcon := d.Object(ui.ID(profileIconID), ui.DescriptionContains(profileIconDes))
	if err := profileIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("profileIcon doesn't exist and skipped logout: ", err)
		return
	} else if err := profileIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click profileIcon: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ClassName(settingsIconClassName), ui.DescriptionContains(settingsIconDescription))
	if err := settingsIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SettingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}
	// Select signed in option as arcapps.
	selectSignedInOption := d.Object(ui.ClassName(selectSignedInOptionClassName), ui.TextMatches("(?i)"+selectSignedInOptionText))
	if err := selectSignedInOption.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SelectSignedInOption doesn't exist: ", err)
	} else if err := selectSignedInOption.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectSignedInOption: ", err)
	}
	// Click on sign out button until email address page exists.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	signOutButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signOutText))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, signOutButton, enterEmailAddress)
}
