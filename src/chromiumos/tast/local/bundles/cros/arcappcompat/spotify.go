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

// clamshellLaunchForSpotify launches Spotify in clamshell mode.
var clamshellLaunchForSpotify = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForSpotify},
}

// touchviewLaunchForSpotify launches Spotify in tablet mode.
var touchviewLaunchForSpotify = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForSpotify},
}

// clamshellAppSpecificTestsForSpotify are placed here.
var clamshellAppSpecificTestsForSpotify = []testutil.TestSuite{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Clamshell: Signout app", Fn: signOutOfSpotify},
}

// touchviewAppSpecificTestsForSpotify are placed here.
var touchviewAppSpecificTestsForSpotify = []testutil.TestSuite{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
	{Name: "Touchview: Signout app", Fn: signOutOfSpotify},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Spotify,
		Desc:         "Functional test for Spotify that installs the app also verifies it is logged in and that the main page is open, checks Spotify correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForSpotify,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForSpotify,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForSpotify,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForSpotify,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForSpotify,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForSpotify,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForSpotify,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForSpotify,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.Spotify.email", "arcappcompat.Spotify.password"},
	})
}

// Spotify test uses library for opting into the playstore and installing app.
// Checks Spotify correctly changes the window states in both clamshell and touchview mode.
func Spotify(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.spotify.music"
		appActivity = ".MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForSpotify verifies Spotify is logged in and
// verify Spotify reached main activity page of the app.
func launchAppForSpotify(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		loginText             = "Log in"
		loginBtnText          = "LOG IN"
		continueWithEmailText = "Continue with Email"
		notNowID              = "android:id/autofill_save_no"
		neverButtonID         = "com.google.android.gms:id/credential_save_reject"
	)

	// Click on login button.
	clickOnLoginButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+loginText))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Fatal("clickOnLoginButton doesn't exist: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
	}

	// Click on Continue with Email button.
	continueWithEmailButton := d.Object(ui.TextMatches("(?i)" + continueWithEmailText))
	if err := continueWithEmailButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueWithEmailButton doesn't exist: ", err)
	} else if err := continueWithEmailButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueWithEmailButton: ", err)
	}

	d.WaitForIdle(ctx, testutil.LongUITimeout)
	// Press tab twice to click on enter email.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Enter email address.
	spotifyEmailID := s.RequiredVar("arcappcompat.Spotify.email")
	if err := kb.Type(ctx, spotifyEmailID); err != nil {
		s.Fatal("Failed to enter enterEmail: ", err)
	}
	s.Log("Entered enterEmail")

	d.WaitForIdle(ctx, testutil.LongUITimeout)
	// Press tab to click on enter password.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
	}

	password := s.RequiredVar("arcappcompat.Spotify.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on Login button.
	loginButton := d.Object(ui.TextMatches("(?i)" + loginBtnText))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Login Button doesn't exist: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}

	// Click on never button.
	neverButton := d.Object(ui.ID(neverButtonID))
	if err := neverButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Never Button doesn't exist: ", err)
	} else if err := neverButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on neverButton: ", err)
	}

	// Click on no thanks button.
	clickOnNoThanksButton := d.Object(ui.ID(notNowID))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}
	// ReLaunch the app to skip save password dialog box.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}

	defer act.Close()

	// Stop the current running activity
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app: ", err)
	}

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}

}

// signOutOfSpotify verifies app is signed out.
func signOutOfSpotify(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		closeID               = "com.spotify.music:id/btn_close"
		settingsIconClassName = "android.widget.ImageButton"
		settingsIconDes       = "Settings"
		scrollLayoutID        = "android:id/list"
		logoutID              = "android:id/text1"
		logOutOfSpotifyText   = "Log out"
		homeiconID            = "com.spotify.music:id/home_tab"
	)
	// Check for homeIcon.
	homeIcon := d.Object(ui.ID(homeiconID))
	if err := homeIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("homeIcon doesn't exist and skipped logout: ", err)
		return
	}
	// Click on close icon to close pop up.
	closeIcon := d.Object(ui.ID(closeID))
	if err := closeIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("closeIcon doesn't exist: ", err)
	} else if err := closeIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on closeIcon: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ClassName(settingsIconClassName), ui.Description(settingsIconDes))
	if err := settingsIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SettingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}
	// Scroll until logout is visible.
	scrollLayout := d.Object(ui.ID(scrollLayoutID), ui.Scrollable(true))
	if err := scrollLayout.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LogOutOfSpotify doesn't exist: ", err)
	}

	logOutOfSpotify := d.Object(ui.ID(logoutID), ui.Text(logOutOfSpotifyText))
	scrollLayout.ScrollTo(ctx, logOutOfSpotify)
	if err := logOutOfSpotify.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LogOutOfSpotify doesn't exist: ", err)
	}
	// Click on log out of Spotify.
	logOutOfSpotify = d.Object(ui.ID(logoutID), ui.Text(logOutOfSpotifyText))
	if err := logOutOfSpotify.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogOutOfSpotify doesn't exist: ", err)
	} else if err := logOutOfSpotify.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfSpotify: ", err)
	}
}
