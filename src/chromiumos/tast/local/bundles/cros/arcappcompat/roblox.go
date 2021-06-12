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

// clamshellLaunchForRoblox launches Roblox in clamshell mode.
var clamshellLaunchForRoblox = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForRoblox},
}

// touchviewLaunchForRoblox launches Roblox in tablet mode.
var touchviewLaunchForRoblox = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForRoblox},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Roblox,
		Desc:         "Functional test for Roblox that installs the app also verifies it is logged in and that the main page is open, checks Roblox correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForRoblox,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForRoblox,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForRoblox,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForRoblox,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Roblox.username", "arcappcompat.Roblox.password"},
	})
}

// Roblox test uses library for opting into the playstore and installing app.
// Checks Roblox correctly changes the window states in both clamshell and touchview mode.
func Roblox(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForRoblox verifies Roblox is logged in and
// verify Roblox reached main activity page of the app.
func launchAppForRoblox(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		loginPageClassName = "android.widget.FrameLayout"
		enterUserNameID    = "com.roblox.client:id/view_login_username_field"
		enterPasswordText  = "Password"
		loginButtonText    = "Log In"
		loginText          = "Login"
		passwordID         = "com.roblox.client:id/view_login_password_field"
	)
	// Check if login page is in web view.
	// If login page is in web view, the test will skip the login part.
	checkForloginPage := d.Object(ui.ClassName(loginPageClassName))
	if err := checkForloginPage.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("checkForloginButtonPage in web view does not exist: ", err)
	} else {
		s.Log("checkForloginButtonPage in web view does exist")
		return
	}
	// Click on login button.
	loginButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(loginText))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Login Button doesn't exists: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}

	// Enter username.
	robloxUserName := s.RequiredVar("arcappcompat.Roblox.username")
	enterUserName := d.Object(ui.ID(enterUserNameID))
	if err := enterUserName.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("enterUserName doesn't exist: ", err)
	} else if err := enterUserName.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterUserName: ", err)
	} else if err := enterUserName.SetText(ctx, robloxUserName); err != nil {
		s.Fatal("Failed to enterUserName: ", err)
	}

	// Enter Password.
	robloxPassword := s.RequiredVar("arcappcompat.Roblox.password")
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if err := enterPassword.SetText(ctx, robloxPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on login button.
	loginButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(loginButtonText))
	if err := loginButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("Login Button doesn't exists: ", err)
	} else if err := loginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on loginButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
