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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForFacebook launches Facebook in clamshell mode.
var clamshellLaunchForFacebook = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForFacebook, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForFacebook launches Facebook in tablet mode.
var touchviewLaunchForFacebook = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForFacebook, Timeout: testutil.LaunchTestCaseTimeout},
}

// clamshellAppSpecificTestsForFacebook are placed here.
var clamshellAppSpecificTestsForFacebook = []testutil.TestCase{
	{Name: "Clamshell: Signout app", Fn: signOutOfFacebook, Timeout: testutil.SignoutTestCaseTimeout},
}

// touchviewAppSpecificTestsForFacebook are placed here.
var touchviewAppSpecificTestsForFacebook = []testutil.TestCase{
	{Name: "Touchview: Signout app", Fn: signOutOfFacebook, Timeout: testutil.SignoutTestCaseTimeout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Facebook,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Facebook that installs the app also verifies it is logged in and that the main page is open, checks Facebook correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForFacebook,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForFacebook,
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
				LaunchTests:      touchviewLaunchForFacebook,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForFacebook,
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
				LaunchTests:      clamshellLaunchForFacebook,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForFacebook,
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
				LaunchTests:      touchviewLaunchForFacebook,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForFacebook,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "clamshell_mode_release",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForFacebook,
				ReleaseTests:     testutil.ClamshellReleaseTests,
				AppSpecificTests: clamshellAppSpecificTestsForFacebook,
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
				LaunchTests:      touchviewLaunchForFacebook,
				ReleaseTests:     testutil.TouchviewReleaseTests,
				AppSpecificTests: touchviewAppSpecificTestsForFacebook,
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
				LaunchTests:      clamshellLaunchForFacebook,
				ReleaseTests:     testutil.ClamshellReleaseTests,
				AppSpecificTests: clamshellAppSpecificTestsForFacebook,
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
				LaunchTests:      touchviewLaunchForFacebook,
				ReleaseTests:     testutil.TouchviewReleaseTests,
				AppSpecificTests: touchviewAppSpecificTestsForFacebook,
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
		VarDeps: []string{"arcappcompat.Facebook.emailid", "arcappcompat.Facebook.password"},
	})
}

// Facebook test uses library for opting into the playstore and installing app.
// Checks Facebook correctly changes the window states in both clamshell and touchview mode.
func Facebook(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.facebook.katana"
		appActivity = ".LoginActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForFacebook verifies Facebook is logged in and
// verify Facebook reached main activity page of the app.
func launchAppForFacebook(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowDes          = "Allow"
		allowText         = "ALLOW"
		addEmailtext      = "Add email"
		cancelID          = "com.google.android.gms:id/cancel"
		contInEngDes      = "Continue in English (US)"
		dismissButtonText = "Dismiss"
		viewGrpClassName  = "android.view.ViewGroup"
		notNowText        = "Not Now"
		okText            = "OK"
		pwdWord           = "Password"
		userNameDes       = "Username"
		skipBtnText       = "Skip"
		textClassName     = "android.widget.EditText"

		waitForActiveInputTime = time.Second * 10
	)

	// Click on cancel button to sign in with google.
	clickOnCancelButton := d.Object(ui.ID(cancelID))
	if err := clickOnCancelButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("clickOnCancelButton doesn't exist: ", err)
	} else if err := clickOnCancelButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnCancelButton: ", err)
	}

	// Click on enter phone or email address field.
	enterEmailAddr := uidetection.TextBlock([]string{"Phone", "or", "email"})
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for enter phone or email",
		ud.WaitUntilExists(enterEmailAddr),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(enterEmailAddr),
		action.Sleep(testutil.DefaultUITimeout),
	)(ctx); err != nil {
		s.Log("Failed to find enter phone or email: ", err)
	}

	// Click on enter mobile number or email address field.
	enterEmailAddr = uidetection.TextBlock([]string{"Mobile", "number", "or", "email"})
	ud = uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for enter mobile number or email",
		ud.WaitUntilExists(enterEmailAddr),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(enterEmailAddr),
		action.Sleep(testutil.DefaultUITimeout),
	)(ctx); err != nil {
		s.Log("Failed to find enter mobile number or email: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Enter email address.
	facebookEmailID := s.RequiredVar("arcappcompat.Facebook.emailid")
	if err := kb.Type(ctx, facebookEmailID); err != nil {
		s.Fatal("Failed to enter enterEmail: ", err)
	}
	s.Log("Entered email")

	// Click on password field
	enterPwd := uidetection.Word(pwdWord).First()
	ud = uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for enter password",
		ud.WaitUntilExists(enterPwd),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(enterPwd),
		action.Sleep(testutil.DefaultUITimeout),
	)(ctx); err != nil {
		s.Log("Failed to enter password: ", err)
	}

	// Enter password.
	facebookPWD := s.RequiredVar("arcappcompat.Facebook.password")
	if err := kb.Type(ctx, facebookPWD); err != nil {
		s.Fatal("Failed to enter enterEmail: ", err)
	}
	s.Log("Entered password")

	// Click on Log in button.
	loginBtn := uidetection.TextBlock([]string{"Log", "In"})
	ud = uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for login button",
		ud.WaitUntilExists(loginBtn),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(loginBtn),
	)(ctx); err != nil {
		s.Log("Failed to find login button: ", err)
	}

	// Click on skip button or not now button for saving login info.
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Click on allow button.
	allowButton := d.Object(ui.DescriptionMatches("(?i)" + allowDes))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on skip button to skip adding a phone number.
	// Click on allow to enable location.
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Click on add email.
	addEmailBtn := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+addEmailtext))
	if err := addEmailBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("addEmailBtn doesn't exist: ", err)
	} else if err := addEmailBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on addEmailBtn: ", err)
	}

	// Click on continue in english.
	contInEngBtn := d.Object(ui.ClassName(viewGrpClassName), ui.DescriptionMatches("(?i)"+contInEngDes))
	if err := contInEngBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("contInEngBtn doesn't exist: ", err)
	} else if err := contInEngBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on contInEngBtn: ", err)
	}

	// Click on skip button to find friends.
	// Click on ok button to turn on device location.
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}

// signOutOfFacebook verifies app is signed out.
func signOutOfFacebook(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		logoutText             = "LOG OUT"
		viewClassName          = "android.view.View"
		scrollClassName        = "androidx.recyclerview.widget.RecyclerView"
		logoutClassName        = "android.view.ViewGroup"
		logoutDes              = "Log out"
		hamburgerIconClassName = "android.view.View"
		notNowText             = "Not Now"
	)
	var indexNum = 5

	// Check for hamburgerIcon.
	hamburgerIcon := d.Object(ui.ClassName(hamburgerIconClassName), ui.Index(indexNum))
	if err := hamburgerIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("hamburgerIcon doesn't exist and skipped logout: ", err)
		return
	} else if err := hamburgerIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on hamburgerIcon: ", err)
	}

	// Scroll until logout is visible.
	scrollLayout := d.Object(ui.ClassName(scrollClassName), ui.Scrollable(true))
	if err := scrollLayout.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LogOutOfFacebook doesn't exist: ", err)
	}

	logOutOfFacebook := d.Object(ui.ClassName(logoutClassName), ui.DescriptionMatches("(?i)"+logoutDes))
	scrollLayout.ScrollTo(ctx, logOutOfFacebook)
	if err := logOutOfFacebook.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("logOutOfFacebook doesn't exist and skipped logout: ", err)
		return
	}

	// Click on log out of Facebook.
	if err := logOutOfFacebook.Exists(ctx); err != nil {
		s.Error("LogOutOfFacebook doesn't exist: ", err)
	} else if err := logOutOfFacebook.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfFacebook: ", err)
	}

	// Click on not now button to save login info.
	notNowButton := d.Object(ui.TextMatches("(?i)" + notNowText))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on log out button.
	logoutBtn := d.Object(ui.TextMatches("(?i)" + logoutText))
	if err := logoutBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("logoutBtn doesn't exist: ", err)
	} else if err := logoutBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on logoutBtn: ", err)
	}
}
