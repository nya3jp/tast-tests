// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForDisney launches Disney in clamshell mode.
var clamshellLaunchForDisney = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForDisney},
}

// touchviewLaunchForDisney launches Disney in tablet mode.
var touchviewLaunchForDisney = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForDisney},
}

// clamshellAppSpecificTestsForDisney are placed here.
var clamshellAppSpecificTestsForDisney = []testutil.TestCase{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Clamshell: Signout app", Fn: signOutOfDisney},
}

// touchviewAppSpecificTestsForDisney are placed here.
var touchviewAppSpecificTestsForDisney = []testutil.TestCase{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Touchview: Signout app", Fn: signOutOfDisney},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Disney,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Disney that installs the app also verifies it is logged in, and that the main page is open, checks Disney correctly changes the window state in both clamshell and touchview mode, finally logout from the app",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForDisney,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForDisney,
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
				LaunchTests:      touchviewLaunchForDisney,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForDisney,
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
				LaunchTests:      clamshellLaunchForDisney,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForDisney,
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
				LaunchTests:      touchviewLaunchForDisney,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForDisney,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "clamshell_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForDisney,
				TopAppTests:      testutil.ClamshellTopAppTests,
				AppSpecificTests: clamshellAppSpecificTestsForDisney,
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
				LaunchTests:      touchviewLaunchForDisney,
				TopAppTests:      testutil.TouchviewTopAppTests,
				AppSpecificTests: touchviewAppSpecificTestsForDisney,
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
				LaunchTests:      clamshellLaunchForDisney,
				TopAppTests:      testutil.ClamshellTopAppTests,
				AppSpecificTests: clamshellAppSpecificTestsForDisney,
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
				LaunchTests:      touchviewLaunchForDisney,
				TopAppTests:      testutil.TouchviewTopAppTests,
				AppSpecificTests: touchviewAppSpecificTestsForDisney,
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
		VarDeps: []string{"arcappcompat.Disney.emailid", "arcappcompat.Disney.password"},
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
		loginID              = "com.disney.disneyplus:id/welcomeButtonLogIn"
		editFieldID          = "com.disney.disneyplus:id/editFieldEditText"
		continueID           = "com.disney.disneyplus:id/continueLoadingButton"
		signInID             = "com.disney.disneyplus:id/standardButtonBackground"
		notNowID             = "android:id/autofill_save_no"
		profileIconClassName = "android.view.ViewGroup"
	)
	var profileIconIndex = 3

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

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	deviceMode := "clamshell"
	if tabletModeEnabled {
		deviceMode = "tablet"
		// Press back to make continue button visible.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Failed to enter KEYCODE_BACK: ", err)
		} else {
			s.Log("Entered KEYCODE_BACK")
		}
	}
	s.Logf("device %v mode", deviceMode)

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

	deviceMode = "clamshell"
	if tabletModeEnabled {
		deviceMode = "tablet"
		// Press back to make continue button visible.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
			s.Log("Failed to enter KEYCODE_BACK: ", err)
		} else {
			s.Log("Entered KEYCODE_BACK")
		}
	}
	s.Logf("device %v mode", deviceMode)
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

	testutil.HandleSavePasswordToGoogle(ctx, s, tconn, a, d, appPkgName)

	// Click on profile icon
	profileIcon := d.Object(ui.ClassName(profileIconClassName), ui.Index(profileIconIndex))
	if err := profileIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("profileIcon doesn't exists: ", err)
	} else if err := profileIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on profileIcon: ", err)
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
		loginID        = "com.disney.disneyplus:id/welcomeButtonLogIn"
		myStuffDes     = "More options. access watchlist, settings, and change profiles."
		scrollLayoutID = "androidx.recyclerview.widget.RecyclerView"
		signOutID      = "com.disney.disneyplus:id/title"
		signOutText    = "Log Out"
	)

	// Click on my stuff icon.
	myStuffIcon := d.Object(ui.DescriptionMatches("(?i)" + myStuffDes))
	if err := myStuffIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("MyStuffIcon doesn't exist and skipped login: ", err)
		return
	} else if err := myStuffIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click MyStuffIcon: ", err)
	}

	// Scroll until signout is visible.
	signOutButton := d.Object(ui.ID(signOutID), ui.TextMatches("(?i)"+signOutText))
	scrollLayout := d.Object(ui.ID(scrollLayoutID), ui.Scrollable(true))
	if err := scrollLayout.WaitForExists(ctx, testutil.DefaultUITimeout); err == nil {
		s.Log("scrollLayout does exist: ", err)
		scrollLayout.ScrollTo(ctx, signOutButton)
	}
	// Click on sign out until the app login button is visible.
	loginButton := d.Object(ui.ID(loginID))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, signOutButton, loginButton)
}
