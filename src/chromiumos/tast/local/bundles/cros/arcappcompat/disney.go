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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForDisney = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForDisney},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: reOpenWindowForDisneyAndSignout},
}

// TouchviewTests are placed here.
var touchviewTestsForDisney = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForDisney},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: reOpenWindowForDisneyAndSignout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Disney,
		Desc:         "Functional test for Disney that installs the app also verifies it is logged in and that the main page is open, checks Disney correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForDisney,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForDisney,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForDisney,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForDisney,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Disney.emailid", "arcappcompat.Disney.password"},
	})
}

// Disney test uses library for opting into the playstore and installing app.
// Checks Disney correctly changes the window states in both clamshell and touchview mode.
func Disney(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.disney.disneyplus"
		appActivity = "com.bamtechmedia.dominguez.main.MainActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForDisney verifies Disney is logged in and
// verify Disney reached main activity page of the app.
func launchAppForDisney(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		loginID     = "com.disney.disneyplus:id/welcomeButtonLogIn"
		editFieldID = "com.disney.disneyplus:id/editFieldEditText"
		continueID  = "com.disney.disneyplus:id/continueLoadingButton"
		signInID    = "com.disney.disneyplus:id/standardButtonBackground"
		homeID      = "com.disney.disneyplus:id/menuViewLayout"
		homeDes     = "Home"
		notNowText  = "NOT NOW"
	)

	// Click on login button.
	loginButton := d.Object(ui.ID(loginID))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Login Button doesn't exists: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(editFieldID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterEmailAddress does not exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.Disney.emailid")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter emailid: ", err)
	}
	s.Log("Entered emailid")

	// Click on continue button.
	continueButton := d.Object(ui.ID(continueID))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Continue Button doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Enter password.
	enterPassword := d.Object(ui.ID(editFieldID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterPassword does not exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	kbp, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kbp.Close()

	password := s.RequiredVar("arcappcompat.Disney.password")
	if err := kbp.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Keep clicking signIn Button until home icon exist in the home page.
	signInButton := d.Object(ui.ID(signInID))
	homeIcon := d.Object(ui.ID(homeID), ui.Description(homeDes))

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := homeIcon.Exists(ctx); err != nil {
			signInButton.Click(ctx)

			// Click on not now button.
			notNowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(notNowText))
			if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
				s.Log("notNowButton doesn't exists: ", err)
			} else if err := notNowButton.Click(ctx); err != nil {
				s.Fatal("Failed to click on notNowButton: ", err)
			}
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Error("HomeIcon doesn't exist: ", err)
	} else {
		s.Log("HomeIcon does exist ")
	}
}

// reOpenWindowForDisneyAndSignout Test "close and relaunch the app" and signout from the app.
func reOpenWindowForDisneyAndSignout(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	s.Log("Close the app")
	if err := a.Command(ctx, "am", "force-stop", appPkgName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to close the app: ", err)
	}
	testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	// Create an app activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
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

	signOutOfDisney(ctx, s, a, d, appPkgName, appActivity)
}

// signOutOfDisney verifies app is signed out.
func signOutOfDisney(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		myStuffDes  = "More options. access watchlist, settings, and change profiles."
		signOutID   = "com.disney.disneyplus:id/title"
		signOutText = "Log Out"
	)

	// Click on my stuff icon.
	myStuffIcon := d.Object(ui.Description(myStuffDes))
	if err := myStuffIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("MyStuffIcon doesn't exist: ", err)
	} else if err := myStuffIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click MyStuffIcon: ", err)
	}

	// Click on sign out button.
	signOutButton := d.Object(ui.ID(signOutID), ui.Text(signOutText))
	if err := signOutButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SignOutButton doesn't exist: ", err)
	} else if err := signOutButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signOutButton: ", err)
	}
}
