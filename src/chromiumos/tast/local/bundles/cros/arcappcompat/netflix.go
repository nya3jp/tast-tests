// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForNetflix launches Netflix in clamshell mode.
var clamshellLaunchForNetflix = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForNetflix},
}

// touchviewLaunchForNetflix launches Netflix in tablet mode.
var touchviewLaunchForNetflix = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForNetflix},
}

// clamshellAppSpecificTestsForNetflix are placed here.
var clamshellAppSpecificTestsForNetflix = []testutil.TestCase{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Clamshell: Signout app", Fn: signOutOfNetflix},
}

// touchviewAppSpecificTestsForNetflix are placed here.
var touchviewAppSpecificTestsForNetflix = []testutil.TestCase{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Touchview: Signout app", Fn: signOutOfNetflix},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Netflix,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Netflix that installs the app also verifies it is logged in and that the main page is open, checks Netflix correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		// TODO(b/186611037): Add Netflix to "appcompat_smoke" suite once the issue mentioned in the comment #5 is resolved.
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForNetflix,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForNetflix,
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
				LaunchTests:      touchviewLaunchForNetflix,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForNetflix,
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
				LaunchTests:      clamshellLaunchForNetflix,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForNetflix,
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
				LaunchTests:      touchviewLaunchForNetflix,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForNetflix,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		},
			/* Disabled due to <1% pass rate over 30 days. See b/246818647
			{
				Name: "clamshell_mode_top_apps",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForNetflix,
					TopAppTests:      testutil.ClamshellTopAppTests,
					AppSpecificTests: clamshellAppSpecificTestsForNetflix,
				},
				ExtraAttr:         []string{"appcompat_top_apps"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			},
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/246818647
			{

				Name: "tablet_mode_top_apps",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForNetflix,
					TopAppTests:      testutil.TouchviewTopAppTests,
					AppSpecificTests: touchviewAppSpecificTestsForNetflix,
				},
				ExtraAttr:         []string{"appcompat_top_apps"},
				ExtraSoftwareDeps: []string{"android_p"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/246818647
			{
				Name: "vm_clamshell_mode_top_apps",
				Val: testutil.TestParams{
					LaunchTests:      clamshellLaunchForNetflix,
					TopAppTests:      testutil.ClamshellTopAppTests,
					AppSpecificTests: clamshellAppSpecificTestsForNetflix,
				},
				ExtraAttr:         []string{"appcompat_top_apps"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on tablet only models.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
				Pre:               pre.AppCompatBootedUsingTestAccountPool,
			},
			*/
			{

				Name: "vm_tablet_mode_top_apps",
				Val: testutil.TestParams{
					LaunchTests:      touchviewLaunchForNetflix,
					TopAppTests:      testutil.TouchviewTopAppTests,
					AppSpecificTests: touchviewAppSpecificTestsForNetflix,
				},
				ExtraAttr:         []string{"appcompat_top_apps"},
				ExtraSoftwareDeps: []string{"android_vm"},
				// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
				// Skip on clamshell only models.
				ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
				Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
			}},
		Timeout: 20 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.Netflix.emailid", "arcappcompat.Netflix.password"},
	})
}

// Netflix test uses library for opting into the playstore and installing app.
// Checks Netflix correctly changes the window states in both clamshell and touchview mode.
func Netflix(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.netflix.mediaclient"
		appActivity = ".ui.launch.UIWebViewActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForNetflix verifies Netflix is logged in and
// verify Netflix reached main activity page of the app.
func launchAppForNetflix(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInButtonText      = "SIGN IN"
		TextClassName         = "android.widget.EditText"
		enterEmailAddressText = "Email or phone number"
		passwordText          = "Password"
		signInBtnText         = "Sign In"
		selectUserID          = "com.netflix.mediaclient:id/2131429149"
		okButtonText          = "OK"
		notNowID              = "android:id/autofill_save_no"
		neverButtonText       = "Never"
		// The inputs rendered by Netflix are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 10
	)

	// Check for signInButton.
	signInButton := d.Object(ui.TextMatches("(?i)" + signInButtonText))
	if err := signInButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("signInButton doesn't exist: ", err)
	}

	signInButton = d.Object(ui.TextMatches("(?i)" + signInButtonText))
	notNowButton := d.Object(ui.ID(notNowID))
	// Click on signIn button until not now button exists.
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, signInButton, notNowButton)

	// Click on not now button.
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on never button.
	neverButton := d.Object(ui.TextMatches("(?i)" + neverButtonText))
	if err := neverButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Never Button doesn't exist: ", err)
	} else if err := neverButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on neverButton: ", err)
	}

	// Enter email address.
	NetflixEmailID := s.RequiredVar("arcappcompat.Netflix.emailid")
	enterEmailAddress := d.Object(ui.ClassName(TextClassName), ui.Text(enterEmailAddressText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if enterEmailAddress.SetText(ctx, NetflixEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter Password.
	NetflixPassword := s.RequiredVar("arcappcompat.Netflix.password")
	enterPassword := d.Object(ui.ClassName(TextClassName), ui.Text(passwordText))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if enterPassword.SetText(ctx, NetflixPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on sign in button again.
	clickOnSignInButton := d.Object(ui.Text(signInBtnText))
	if err := clickOnSignInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SignInButton doesn't exist: ", err)
	} else if err := clickOnSignInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnSignInButton: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Click on never button to save your password.
	if err := neverButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Never Button doesn't exist: ", err)
	} else if err := neverButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on neverButton: ", err)
	}

	userProfileButton := uidetection.Word("appcompat")
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for user profile",
		ud.WaitUntilExists(userProfileButton),
		ud.Tap(userProfileButton),
		action.Sleep(waitForActiveInputTime),
	)(ctx); err != nil {
		s.Fatal("Failed to find user profile: ", err)
	}

	// Click on ok button.
	okButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(okButtonText))
	if err := okButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("okButton doesn't exist: ", err)
	} else if err := okButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on okButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}

// signOutOfNetflix verifies app is signed out.
func signOutOfNetflix(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		downloadID            = "com.netflix.mediaclient:id/smart_downloads_icon"
		enterEmailAddressText = "Email or phone number"
		layoutClassName       = "android.widget.FrameLayout"
		hamburgerIconDes      = "More"
		homeIconID            = "com.netflix.mediaclient:id/ribbon_n_logo"
		scrollLayoutClassName = "android.widget.ScrollView"
		signOutButtonID       = "com.netflix.mediaclient:id/row_text"
		signOutText           = "Sign Out"
		selectUserID          = "com.netflix.mediaclient:id/2131429149"
		TextClassName         = "android.widget.EditText"
		// The inputs rendered by Netflix are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 10
	)

	userProfileButton := uidetection.Word("appcompat")
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for user profile",
		ud.WaitUntilExists(userProfileButton),
		ud.Tap(userProfileButton),
		action.Sleep(waitForActiveInputTime),
	)(ctx); err != nil {
		s.Fatal("Failed to find user profile: ", err)
	}

	// Check for Introducing downloads pop up
	checkForIntroDownloads := d.Object(ui.ID(downloadID))
	if err := checkForIntroDownloads.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("checkForIntroDownloads doesn't exist: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_BACK: ", err)
	}

	clickOnHamburgerIcon := d.Object(ui.ClassName(layoutClassName), ui.Description(hamburgerIconDes))
	if err := clickOnHamburgerIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("ClickOnHamburgerIcon doesn't exist: ", err)
	} else if err := clickOnHamburgerIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnHamburgerIcon: ", err)
	}

	// Click on hamburgerIcon button until scroll layout exists.
	signOutButton := d.Object(ui.TextMatches("(?i)" + signOutText))
	scrollLayout := d.Object(ui.ClassName(scrollLayoutClassName), ui.Scrollable(true))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, signOutButton, scrollLayout)
	if err := scrollLayout.Exists(ctx); err == nil {
		scrollLayout.ScrollTo(ctx, signOutButton)
	}

	// Click on sign out button.
	if err := signOutButton.Exists(ctx); err != nil {
		s.Error("signOutButton doesn't exist: ", err)
	} else if err := signOutButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signOutButton: ", err)
	}

	// Click on sign out of Netflix.
	signOutOfNetflix := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signOutText))
	if err := signOutOfNetflix.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("signOutOfNetflix doesn't exist: ", err)
	} else if err := signOutOfNetflix.Click(ctx); err != nil {
		s.Fatal("Failed to click on signOutOfNetflix: ", err)
	}

	// Click on sign out button until enterEmailAddress page exists.
	enterEmailAddress := d.Object(ui.ClassName(TextClassName), ui.Text(enterEmailAddressText))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, signOutOfNetflix, enterEmailAddress)
}
