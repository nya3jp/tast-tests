// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForNetflix = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForNetflix},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForNetflix = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForNetflix},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Netflix,
		Desc:         "Functional test for Netflix that installs the app also verifies it is logged in and that the main page is open, checks Netflix correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForNetflix,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForNetflix,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForNetflix,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForNetflix,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
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

	// Step up chrome on Chromebook.
	cr, tconn, a, d := testutil.SetUpDevice(ctx, s, appPkgName, appActivity)
	defer d.Close()

	testSet := s.Param().(testutil.TestParams)
	// Run the different test cases.
	for idx, test := range testSet.Tests {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			// Launch the app.
			act, err := arc.NewActivity(a, appPkgName, appActivity)
			if err != nil {
				s.Fatal("Failed to create new app activity: ", err)
			}
			s.Log("Created new app activity")

			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}

			defer act.Stop(ctx, tconn)

			defer func() {
				if s.HasError() {
					filename := fmt.Sprintf("screenshot-arcappcompat-failed-test-%d.png", idx)
					path := filepath.Join(s.OutDir(), filename)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
				}
			}()

			testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForNetflix verifies Netflix is logged in and
// verify Netflix reached main activity page of the app.
func launchAppForNetflix(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInButtonDes       = "SIGN IN"
		TextClassName         = "android.widget.EditText"
		enterEmailAddressText = "Email or phone number"
		passwordText          = "Password"
		signInBtnText         = "Sign In"
		selectUserID          = "com.netflix.mediaclient:id/profile_avatar_title"
		okButtonText          = "OK"
		homeIconID            = "com.netflix.mediaclient:id/ribbon_n_logo"
	)
	var selectUserIndex int

	if currentAppPkg := testutil.CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("Failed to launch the app: ", currentAppPkg)
	}
	s.Log("App is launched successfully in launchAppForNetflix")

	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Description(signInButtonDes))
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

	// Select User.
	selectUser := d.Object(ui.ID(selectUserID), ui.Index(selectUserIndex))
	if err := selectUser.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SelectUser doesn't exist: ", err)
	} else if err := selectUser.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectUser: ", err)
	}

	// Click on ok button.
	okButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(okButtonText))
	if err := okButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("okButton doesn't exist: ", err)
	} else if err := selectUser.Click(ctx); err != nil {
		s.Fatal("Failed to click on okButton: ", err)
	}

	// Check for home icon.
	homeIcon := d.Object(ui.ID(homeIconID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("SelectUser doesn't exist: ", err)
	}

	signOutOfNetflix(ctx, s, a, d, appPkgName, appActivity)

}

// signOutOfNetflix verifies app is signed out.
func signOutOfNetflix(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		layoutClassName  = "android.widget.FrameLayout"
		hamburgerIconDes = "More"
		signOutButtonID  = "com.netflix.mediaclient:id/row_text"
		signOutText      = "Sign Out"
	)

	// Click on hamburger icon.
	clickOnHamburgerIcon := d.Object(ui.ClassName(layoutClassName), ui.Description(hamburgerIconDes))
	if err := clickOnHamburgerIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("ClickOnHamburgerIcon doesn't exist: ", err)
	} else if err := clickOnHamburgerIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnHamburgerIcon: ", err)
	}

	// Click on sign out button.
	signOutButton := d.Object(ui.ID(signOutButtonID), ui.Text(signOutText))
	if err := signOutButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
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
