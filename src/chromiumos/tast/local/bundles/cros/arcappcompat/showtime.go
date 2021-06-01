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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// ClamshellTests are placed here.
var clamshellTestsForShowtime = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForShowtime},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForShowtime = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForShowtime},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Showtime,
		Desc:         "Functional test for Showtime that installs the app also verifies it is logged in and that the main page is open, checks Showtime correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForShowtime,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForShowtime,
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("careena"), hwdep.SkipOnModel("kasumi"), hwdep.SkipOnModel("treeya"),
				hwdep.SkipOnModel("bluebird"), hwdep.SkipOnModel("sarien"), hwdep.SkipOnModel("elemi"), hwdep.SkipOnModel("berknip"),
				hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("aleena"), hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("grunt"),
				hwdep.SkipOnModel("liara"), hwdep.SkipOnModel("nuwani"), hwdep.SkipOnModel("kindred"), hwdep.SkipOnModel("dratini"),
				hwdep.SkipOnModel("apel"), hwdep.SkipOnModel("blooglet"), hwdep.SkipOnModel("blorb"), hwdep.SkipOnModel("bobba"),
				hwdep.SkipOnModel("casta"), hwdep.SkipOnModel("dorp"), hwdep.SkipOnModel("droid"), hwdep.SkipOnModel("fleex"),
				hwdep.SkipOnModel("foob"), hwdep.SkipOnModel("garfour"), hwdep.SkipOnModel("garg"), hwdep.SkipOnModel("laser14"),
				hwdep.SkipOnModel("lick"), hwdep.SkipOnModel("mimrock"), hwdep.SkipOnModel("nospike"), hwdep.SkipOnModel("orbatrix"),
				hwdep.SkipOnModel("phaser"), hwdep.SkipOnModel("sparky")),
			Pre: pre.AppCompatBootedInTabletMode,
		}, {
			Name:              "vm",
			Val:               clamshellTestsForShowtime,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForShowtime,
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("careena"), hwdep.SkipOnModel("kasumi"), hwdep.SkipOnModel("treeya"),
				hwdep.SkipOnModel("bluebird"), hwdep.SkipOnModel("sarien"), hwdep.SkipOnModel("elemi"), hwdep.SkipOnModel("berknip"),
				hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("aleena"), hwdep.SkipOnModel("barla"), hwdep.SkipOnModel("grunt"),
				hwdep.SkipOnModel("liara"), hwdep.SkipOnModel("nuwani"), hwdep.SkipOnModel("kindred"), hwdep.SkipOnModel("dratini"),
				hwdep.SkipOnModel("apel"), hwdep.SkipOnModel("blooglet"), hwdep.SkipOnModel("blorb"), hwdep.SkipOnModel("bobba"),
				hwdep.SkipOnModel("casta"), hwdep.SkipOnModel("dorp"), hwdep.SkipOnModel("droid"), hwdep.SkipOnModel("fleex"),
				hwdep.SkipOnModel("foob"), hwdep.SkipOnModel("garfour"), hwdep.SkipOnModel("garg"), hwdep.SkipOnModel("laser14"),
				hwdep.SkipOnModel("lick"), hwdep.SkipOnModel("mimrock"), hwdep.SkipOnModel("nospike"), hwdep.SkipOnModel("orbatrix"),
				hwdep.SkipOnModel("phaser"), hwdep.SkipOnModel("sparky")),
			Pre: pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Showtime.emailid", "arcappcompat.Showtime.password"},
	})
}

// Showtime test uses library for opting into the playstore and installing app.
// Checks Showtime correctly changes the window states in both clamshell and touchview mode.
func Showtime(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.showtime.standalone"
		appActivity = "com.showtime.showtimeanytime.activities.IntroActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForShowtime verifies Showtime is logged in and
// verify Showtime reached main activity page of the app.
func launchAppForShowtime(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signInID        = "com.showtime.standalone:id/loginButton"
		enterEmailID    = "com.showtime.standalone:id/emailLogin"
		enterPasswordID = "com.showtime.standalone:id/passwordLogin"
		logInText       = "SIGN IN"
		logInID         = "com.showtime.standalone:id/signInButton"
		notNowButtonID  = "android:id/autofill_save_no"
		homeIconID      = "com.showtime.standalone:id/text"
		homeIconText    = "HOME"
	)

	// Click on sign in button.
	signInButton := d.Object(ui.ID(signInID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Sign in button doesn't exist: ", err)
	} else if err := signInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Sign in button: ", err)
	}

	// Enter email address.
	ShowtimeEmailID := s.RequiredVar("arcappcompat.Showtime.emailid")
	enterEmailAddress := d.Object(ui.ID(enterEmailID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if err := enterEmailAddress.SetText(ctx, ShowtimeEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter password.
	ShowtimePassword := s.RequiredVar("arcappcompat.Showtime.password")
	enterPassword := d.Object(ui.ID(enterPasswordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, ShowtimePassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on log in button
	logInButton := d.Object(ui.ID(logInID), ui.Text(logInText))
	if err := logInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogIn button doesn't exist: ", err)
	} else if err := logInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on LogIn button: ", err)
	}

	notNowButton := d.Object(ui.ID(notNowButtonID))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton button doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
