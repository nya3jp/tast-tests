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
var clamshellTestsForCanva = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForCanva},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForCanva = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForCanva},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Canva,
		Desc:         "Functional test for Canva that installs the app also verifies it is logged in and that the main page is open, checks Canva correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForCanva,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForCanva,
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
			Val:               clamshellTestsForCanva,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForCanva,
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
			"arcappcompat.Canva.emailid", "arcappcompat.Canva.password"},
	})
}

// Canva test uses library for opting into the playstore and installing app.
// Checks Canva correctly changes the window states in both clamshell and touchview mode.
func Canva(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.canva.editor"
		appActivity = "com.canva.app.editor.splash.SplashActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForCanva verifies Canva is logged in and
// verify Canva reached main activity page of the app.
func launchAppForCanva(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		googleSignInText = "Continue with Google"
		designText       = "What will you be using"
		emailAddressID   = "com.google.android.gms:id/container"
		homeIconText     = "Create a design"
	)

	// Click on sign in button.
	googleSignInButton := d.Object(ui.Text(googleSignInText))
	if err := googleSignInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		// For selecting Gmail account
		s.Log("googleSignInButton doesn't exist and press Tab and Enter: ", err)
		d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0)
		d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0)
	} else if err := googleSignInButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on sign in button: ", err)
	}

	// Choose first recommend design by hitting enter key
	designTextButton := d.Object(ui.TextContains(designText))
	if err := designTextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("designTextButton doesn't exist: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to press ENTER key: ", err)
	}

	// Click on email address.
	emailAddress := d.Object(ui.ID(emailAddressID))
	if err := emailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for homePageVerifier.
	homePageVerifier := d.Object(ui.Text(homeIconText))
	if err := homePageVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("homePageVerifier button doesn't exist: ", err)
	}
}
