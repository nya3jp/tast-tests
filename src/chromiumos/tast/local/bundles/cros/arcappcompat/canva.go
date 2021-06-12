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

// clamshellLaunchForCanva launches Canva in clamshell mode.
var clamshellLaunchForCanva = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForCanva},
}

// touchviewLaunchForCanva launches Canva in tablet mode.
var touchviewLaunchForCanva = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForCanva},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Canva,
		Desc:         "Functional test for Canva that installs the app also verifies it is logged in and that the main page is open, checks Canva correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForCanva,
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
				Tests:      touchviewLaunchForCanva,
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
				Tests:      clamshellLaunchForCanva,
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
				Tests:      touchviewLaunchForCanva,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.Canva.emailid", "arcappcompat.Canva.password"},
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Canva test uses library for opting into the playstore and installing app.
// Checks Canva correctly changes the window states in both clamshell and touchview mode.
func Canva(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.canva.editor"
		appActivity = "com.canva.app.editor.splash.SplashActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
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

	// Click on email address.
	emailAddress := d.Object(ui.ID(emailAddressID))
	if err := emailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	}

	if currentAppPkg, err := testutil.CurrentAppPackage(ctx, d); err != nil {
		s.Fatal("Failed to get current app package: ", err)
	} else if currentAppPkg != appPkgName && currentAppPkg != "com.google.android.packageinstaller" && currentAppPkg != "com.google.android.gms" && currentAppPkg != "com.google.android.permissioncontroller" {
		s.Fatalf("Failed to launch after login: incorrect package(expected: %s, actual: %s)", appPkgName, currentAppPkg)
	}
	testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}
