// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForNetflix launches Netflix in clamshell mode.
var clamshellLaunchForNetflix = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForNetflix},
}

// touchviewLaunchForNetflix launches Netflix in tablet mode.
var touchviewLaunchForNetflix = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForNetflix},
}

// clamshellAppSpecificTestsForNetflix are placed here.
var clamshellAppSpecificTestsForNetflix = []testutil.TestSuite{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Clamshell: Signout app", Fn: signOutOfNetflix},
}

// touchviewAppSpecificTestsForNetflix are placed here.
var touchviewAppSpecificTestsForNetflix = []testutil.TestSuite{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Touchview: Signout app", Fn: signOutOfNetflix},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     Netflix,
		Desc:     "Functional test for Netflix that installs the app also verifies it is logged in and that the main page is open, checks Netflix correctly changes the window state in both clamshell and touchview mode",
		Contacts: []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		// TODO(b/186611037): Add Netflix to "appcompat_smoke" suite once the issue mentioned in the comment #5 is resolved.
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForNetflix,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForNetflix,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForNetflix,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForNetflix,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForNetflix,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForNetflix,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForNetflix,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForNetflix,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Netflix.emailid", "arcappcompat.Netflix.password"},
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
		selectUserID          = "com.netflix.mediaclient:id/profile_avatar_img"
		okButtonText          = "OK"
	)

	// Check for signInButton.
	signInButton := d.Object(ui.TextMatches("(?i)" + signInButtonText))
	if err := signInButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("signInButton doesn't exist: ", err)
	}
	signInButton = d.Object(ui.TextMatches("(?i)" + signInButtonText))
	enterEmailAddress := d.Object(ui.ClassName(TextClassName), ui.Text(enterEmailAddressText))
	// Keep clicking signIn button until enterEmailAddress exist in the home page.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := enterEmailAddress.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("EnterEmailAddress doesn't exist: ", err)
	} else {
		s.Log("EnterEmailAddress does exists")
	}
	// Enter email address.
	NetflixEmailID := s.RequiredVar("arcappcompat.Netflix.emailid")
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

	// Select User.
	selectUser := d.Object(ui.ID(selectUserID), ui.Index(0))
	if err := selectUser.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SelectUser doesn't exist: ", err)
	} else if err := selectUser.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectUser: ", err)
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
		layoutClassName       = "android.widget.FrameLayout"
		hamburgerIconDes      = "More"
		homeIconID            = "com.netflix.mediaclient:id/ribbon_n_logo"
		scrollLayoutClassName = "android.widget.ScrollView"
		signOutButtonID       = "com.netflix.mediaclient:id/row_text"
		signOutText           = "Sign Out"
		selectUserID          = "com.netflix.mediaclient:id/profile_avatar_img"
	)

	// Select User.
	selectUser := d.Object(ui.ID(selectUserID), ui.Index(0))
	if err := selectUser.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("SelectUser doesn't exist: ", err)
	} else if err := selectUser.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectUser: ", err)
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
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := scrollLayout.Exists(ctx); err != nil {
			if err := clickOnHamburgerIcon.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click on HamburgerIcon")
			}
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("scrollLayout doesn't exist: ", err)
	} else if err := scrollLayout.ScrollTo(ctx, signOutButton); err != nil {
		s.Fatal("Failed to scroll: ", err)
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
}
