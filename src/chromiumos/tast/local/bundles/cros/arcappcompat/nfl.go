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
var clamshellTestsForNFL = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForNFL},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForNFL = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForNFL},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         NFL,
		Desc:         "Functional test for NFL that installs the app also verifies it is logged in and that the main page is open, checks NFL correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"archanasing@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               clamshellTestsForNFL,
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "tablet_mode",
			Val:               touchviewTestsForNFL,
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
			Val:               clamshellTestsForNFL,
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name:              "vm_tablet_mode",
			Val:               touchviewTestsForNFL,
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
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// NFL test uses library for opting into the playstore and installing app.
// Checks NFL correctly changes the window states in both clamshell and touchview mode.
func NFL(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.gotv.nflgamecenter.us.lite"
		appActivity = "com.nfl.mobile.SplashActivity"
	)
	testCases := s.Param().([]testutil.TestCase)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testCases)
}

// launchAppForNFL verifies NFL is logged in and
// verify NFL reached main activity page of the app.
func launchAppForNFL(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		skipText = "Skip"
		homeText = "Home"
	)

	// Click on skip button until home page exists.
	skipButton := d.Object(ui.Text(skipText))
	homeButton := d.Object(ui.Text(homeText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := homeButton.Exists(ctx); err != nil {
			skipButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("homeButton doesn't exist: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
