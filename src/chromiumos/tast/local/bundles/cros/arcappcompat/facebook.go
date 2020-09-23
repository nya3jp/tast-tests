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
var clamshellTestsForFacebook = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForFacebook},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForFacebook = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForFacebook},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Facebook,
		Desc:         "Functional test for Facebook that installs the app also verifies it is logged in and that the main page is open, checks Facebook correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForFacebook,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForFacebook,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForFacebook,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForFacebook,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Facebook.emailid", "arcappcompat.Facebook.password"},
	})
}

// Facebook test uses library for opting into the playstore and installing app.
// Checks Facebook correctly changes the window states in both clamshell and touchview mode.
func Facebook(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.facebook.katana"
		appActivity = ".LoginActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForFacebook verifies Facebook is logged in and
// verify Facebook reached main activity page of the app.
func launchAppForFacebook(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowDes               = "Allow"
		allowText              = "ALLOW"
		dismissButtonText      = "Dismiss"
		loginPageClassName     = "android.view.ViewGroup"
		loginPageDes           = "•"
		hamburgerIconClassName = "android.view.View"
		notNowText             = "Not Now"
		notNowID               = "android:id/autofill_save_no"
		okText                 = "OK"
		passwordDes            = "Password"
		userNameDes            = "Username"
		textClassName          = "android.widget.EditText"
	)
	var indexNum = 5

	// Check for login page.
	checkForloginPage := d.Object(ui.ClassName(loginPageClassName), ui.Description(loginPageDes))
	if err := checkForloginPage.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("checkForloginButtonPage does exist")
		return
	}
	// Enter email address.
	FacebookEmailID := s.RequiredVar("arcappcompat.Facebook.emailid")
	enterEmailAddress := d.Object(ui.ClassName(textClassName), ui.Description(userNameDes))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if enterEmailAddress.SetText(ctx, FacebookEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter Password.
	FacebookPassword := s.RequiredVar("arcappcompat.Facebook.password")
	enterPassword := d.Object(ui.ClassName(textClassName), ui.Description(passwordDes))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if enterPassword.SetText(ctx, FacebookPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on login button.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to press enter button: ", err)
	}

	// Click on no thanks button.
	clickOnNoThanksButton := d.Object(ui.ID(notNowID))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}

	// Click on dismiss button.
	clickOnDismissButton := d.Object(ui.Text(dismissButtonText))
	if err := clickOnDismissButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnDismissButton doesn't exist: ", err)
	} else if err := clickOnDismissButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnDismissButton: ", err)
	}

	// Click on not now button for adding contacts.
	clickOnNotNowButton := d.Object(ui.Text(notNowText))
	if err := clickOnNotNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNotNowButton doesn't exist: ", err)
	} else if err := clickOnNotNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNotNowButton: ", err)
	}

	// Click on allow button.
	clickOnAllowButton := d.Object(ui.Description(allowDes))
	if err := clickOnAllowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := clickOnAllowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAllowButton: ", err)
	}

	// Click on allow button again.
	clickOnAllowButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowText))
	if err := clickOnAllowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := clickOnAllowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAllowButton: ", err)
	}

	// Click on ok button to turn on device location.
	clickOnOkButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(okText))
	if err := clickOnOkButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnOkButton doesn't exist: ", err)
	} else if err := clickOnOkButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnOkButton: ", err)
	}

	// Check for hambuger Icon.
	hambugerIcon := d.Object(ui.ClassName(hamburgerIconClassName), ui.Index(indexNum))
	if err := hambugerIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("hambugerIcon doesn't exist: ", err)
	} else {
		signOutOfFacebook(ctx, s, a, d, appPkgName, appActivity)
	}
}

// signOutOfFacebook verifies app is signed out.
func signOutOfFacebook(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		viewClassName   = "android.view.View"
		scrollClassName = "androidx.recyclerview.widget.RecyclerView"
		logoutClassName = "android.view.ViewGroup"
		logoutDes       = "Log Out, Button 1 of 1"
	)
	var indexNum = 5

	// Click on menu icon.
	menuIcon := d.Object(ui.ClassName(viewClassName), ui.Index(indexNum))
	if err := menuIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("menuIcon doesn't exist: ", err)
	} else if err := menuIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on menuIcon: ", err)
	}

	// Scroll until logout is visible.
	scrollLayout := d.Object(ui.ClassName(scrollClassName), ui.Scrollable(true))
	if err := scrollLayout.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LogOutOfFacebook doesn't exist: ", err)
	}

	logOutOfFacebook := d.Object(ui.ClassName(logoutClassName), ui.Description(logoutDes))
	scrollLayout.ScrollTo(ctx, logOutOfFacebook)
	if err := logOutOfFacebook.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("logOutOfFacebook doesn't exist: ", err)
	}

	// Click on log out of Facebook.
	if err := logOutOfFacebook.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogOutOfFacebook doesn't exist: ", err)
	} else if err := logOutOfFacebook.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfFacebook: ", err)
	}
}
