// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForSkype = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSkype},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: reOpenWindowForSkypeAndSignOutOfApp},
}

// TouchviewTests are placed here.
var touchviewTestsForSkype = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSkype},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: reOpenWindowForSkypeAndSignOutOfApp},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Skype,
		Desc:         "Functional test for Skype that installs the app also verifies it is logged in and that the main page is open, checks Skype correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForSkype,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForSkype,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForSkype,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForSkype,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Skype.emailid", "arcappcompat.Skype.password"},
	})
}

// Skype test uses library for opting into the playstore and installing app.
// Checks Skype correctly changes the window states in both clamshell and touchview mode.
func Skype(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.skype.raider"
		appActivity = "com.skype.m2.views.HubActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForSkype verifies Skype is logged in and
// verify Skype reached main activity page of the app.
func launchAppForSkype(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText     = "ALLOW"
		enterEmailAddressID = "i0116"
		hamburgerClassName  = "android.widget.ImageButton"
		hamburgerDes        = "Menu"
		nextButtonText      = "Next"
		passwordID          = "i0118"
		signInClassName     = "android.widget.Button"
		signInText          = "Sign in"
	)
	// Click on sign in button.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signInText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("signInButton doesn't exists: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exists: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	emailAddress := s.RequiredVar("arcappcompat.Skype.emailid")
	if err := enterEmailAddress.SetText(ctx, emailAddress); err != nil {
		s.Fatal("Doesn't enter EmailAddress: ", err)
	}
	s.Log("Entered EmailAddress")

	// Click on next button
	nextButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(nextButtonText))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Next Button doesn't exists: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterPassword doesn't exists: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	// Keep clicking password text field until the password text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
			s.Log("Password text field is focused ")
		} else if !pwdFocused {
			enterPassword.Click(ctx)
			return errors.New("Password text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	kbp, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kbp.Close()

	password := s.RequiredVar("arcappcompat.Skype.password")
	if err := kbp.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on Sign in button.
	signInButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signInText))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SignInButton doesn't exists: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signInButton: ", err)
	}

	// Click on allow button to access your files.
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Check for hamburgerIcon on homePage.
	hamburgerIcon := d.Object(ui.ClassName(hamburgerClassName), ui.Description(hamburgerDes))
	if err := hamburgerIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("hamburgerIcon doesn't exists: ", err)
	}
}

// reOpenWindowForSkypeAndSignOutOfApp Test "close and relaunch the app", verifies app launch successfully without crash or ANR and signout of an app.
func reOpenWindowForSkypeAndSignOutOfApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	// Launch the app.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	s.Log("Created new app activity")

	defer act.Close()

	s.Log("Stop the current activity of the app")
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app: ", err)
	}

	testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	// Create an app activity handle.
	act, err = arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	s.Log("Created new app activity")

	defer act.Close()

	// ReLaunch the activity.
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	s.Log("App relaunched successfully")

	signOutOfSkype(ctx, s, tconn, a, d, appPkgName, appActivity)
}

// signOutOfSkype verifies app is signed out.
func signOutOfSkype(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		closeIconClassName = "android.widget.ImageButton"
		closeIconDes       = "Close main menus"
		hamburgerClassName = "android.widget.ImageButton"
		hamburgerDes       = "Menu"
		signOutID          = "com.skype.raider:id/drawer_signout"
	)

	// Click on close main menu icon
	clickOnCloseIcon := d.Object(ui.ClassName(closeIconClassName), ui.Description(closeIconDes))
	if err := clickOnCloseIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnCloseIcon doesn't exists: ", err)
	} else if err := clickOnCloseIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnCloseIcon: ", err)
	}

	// Check for hamburgerIcon on homePage.
	hamburgerIcon := d.Object(ui.ClassName(hamburgerClassName), ui.Description(hamburgerDes))
	if err := hamburgerIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("hamburgerIcon doesn't exists: ", err)
	} else if err := hamburgerIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on hamburgerIcon: ", err)
	}

	// Click on sign out of Skype.
	signOutOfSkype := d.Object(ui.ID(signOutID))
	if err := signOutOfSkype.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("signOutOfSkype doesn't exist: ", err)
	} else if err := signOutOfSkype.Click(ctx); err != nil {
		s.Fatal("Failed to click on signOutOfSkype: ", err)
	}
}
