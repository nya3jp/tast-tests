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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForDisney launches Disney in clamshell mode.
var clamshellLaunchForDisney = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForDisney},
}

// touchviewLaunchForDisney launches Disney in tablet mode.
var touchviewLaunchForDisney = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForDisney},
}

// clamshellAppSpecificTestsForDisney are placed here.
var clamshellAppSpecificTestsForDisney = []testutil.TestSuite{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Clamshell: Signout app", Fn: signOutOfDisney},
}

// touchviewAppSpecificTestsForDisney are placed here.
var touchviewAppSpecificTestsForDisney = []testutil.TestSuite{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Touchview: Signout app", Fn: signOutOfDisney},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Disney,
		Desc:         "Functional test for Disney that installs the app also verifies it is logged in, and that the main page is open, checks Disney correctly changes the window state in both clamshell and touchview mode, finally logout from the app",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForDisney,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForDisney,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForDisney,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForDisney,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForDisney,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForDisney,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForDisney,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForDisney,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Disney.emailid", "arcappcompat.Disney.password"},
	})
}

// Disney test uses library for opting into the playstore and installing app.
// Checks Disney correctly changes the window states in both clamshell and touchview mode.
func Disney(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.disney.disneyplus"
		appActivity = "com.bamtechmedia.dominguez.main.MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForDisney verifies Disney is logged in and
// verify Disney reached main activity page of the app.
func launchAppForDisney(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		loginID     = "com.disney.disneyplus:id/welcomeButtonLogIn"
		editFieldID = "com.disney.disneyplus:id/editFieldEditText"
		continueID  = "com.disney.disneyplus:id/continueLoadingButton"
		signInID    = "com.disney.disneyplus:id/standardButtonBackground"
		notNowID    = "android:id/autofill_save_no"
	)

	// Click on login button.
	loginButton := d.Object(ui.ID(loginID))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Login Button doesn't exists: ", err)
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
		s.Error("Continue Button doesn't exists: ", err)
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

	// Check for signInButton.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("signInButton doesn't exists: ", err)
	}
	// Click on signIn Button until notNowButton exist.
	signInButton = d.Object(ui.ID(signInID))
	notNowButton := d.Object(ui.ID(notNowID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := notNowButton.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else {
		s.Log("notNowButton does exist ")
	}

	// Click on not now button.
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}

// signOutOfDisney verifies app is signed out.
func signOutOfDisney(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		myStuffDes  = "More options. access watchlist, settings, and change profiles."
		signOutID   = "com.disney.disneyplus:id/title"
		signOutText = "Log Out"
	)

	// Click on my stuff icon.
	myStuffIcon := d.Object(ui.Description(myStuffDes))
	if err := myStuffIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("MyStuffIcon doesn't exist and skipped login: ", err)
		return
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
