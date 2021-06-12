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

// clamshellLaunchForTiktok launches Tiktok in clamshell mode.
var clamshellLaunchForTiktok = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForTiktok},
}

// touchviewLaunchForTiktok launches Tiktok in tablet mode.
var touchviewLaunchForTiktok = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForTiktok},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Tiktok,
		Desc:         "Functional test for Tiktok that installs the app also verifies it is logged in and that the main page is open, checks Tiktok correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForTiktok,
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
				Tests:      touchviewLaunchForTiktok,
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
				Tests:      clamshellLaunchForTiktok,
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
				Tests:      touchviewLaunchForTiktok,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Tiktok test uses library for opting into the playstore and installing app.
// Checks Tiktok correctly changes the window states in both clamshell and touchview mode.
func Tiktok(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.zhiliaoapp.musically"
		appActivity = "com.ss.android.ugc.aweme.splash.SplashActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForTiktok verifies Tiktok is logged in and
// verify Tiktok reached main activity page of the app.
func launchAppForTiktok(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		loginText                      = "Already have an account? Log in"
		loginWithGoogleButtonClassName = "android.view.ViewGroup"
		loginWithPreviousDeviceText    = "Log in with previous device"
		emailAddressID                 = "com.google.android.gms:id/container"
		textviewClassName              = "android.widget.TextView"
		skipText                       = "Skip"
		startWatchingText              = "Start watching"
		signUpButtonID                 = "com.zhiliaoapp.musically:id/ak"
	)
	var (
		loginWithGoogleIndex = 4
		emailAddressIndex    = 0
	)
	// check for log in with previous device.
	checkForLoginWithPreviousDevice := d.Object(ui.ClassName(textviewClassName), ui.TextMatches("(?i)"+loginWithPreviousDeviceText))
	emailAddress := d.Object(ui.ID(emailAddressID), ui.Index(emailAddressIndex))
	if err := checkForLoginWithPreviousDevice.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("checkForLoginWithPreviousDevice doesn't exist: ", err)
		continueWithGoogle := d.Object(ui.TextMatches("(?i)" + "Continue with Google"))
		// Click on continue with Google button until EmailAddress exist.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := emailAddress.Exists(ctx); err != nil {
				continueWithGoogle.Click(ctx)
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
			s.Log("emailAddress doesn't exist: ", err)
		}

	} else {
		s.Log("checkForLoginWithPreviousDevice does exist")
		loginWithGoogleButton := d.Object(ui.ClassName(loginWithGoogleButtonClassName), ui.Index(loginWithGoogleIndex))
		// Click on login with Google Button until EmailAddress exist.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := emailAddress.Exists(ctx); err != nil {
				loginWithGoogleButton.Click(ctx)
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
			s.Log("emailAddress doesn't exist: ", err)
		}
	}

	skipButton := d.Object(ui.ClassName(textviewClassName), ui.TextMatches("(?i)"+skipText))
	emailAddress = d.Object(ui.ID(emailAddressID), ui.Index(emailAddressIndex))
	// Click on EmailAddress until skipButton exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := skipButton.Exists(ctx); err != nil {
			emailAddress.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("skipButton doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}

	// Click on start watching button.
	startWatchingButton := d.Object(ui.ClassName(textviewClassName), ui.TextMatches("(?i)"+startWatchingText))
	if err := startWatchingButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("startWatchingButton doesn't exist: ", err)
	} else if err := startWatchingButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on startWatchingButton: ", err)
	}

	if currentAppPkg, err := testutil.CurrentAppPackage(ctx, d); err != nil {
		s.Fatal("Failed to get current app package: ", err)
	} else if currentAppPkg != appPkgName && currentAppPkg != "com.google.android.packageinstaller" && currentAppPkg != "com.google.android.gms" && currentAppPkg != "com.google.android.permissioncontroller" {
		s.Fatalf("Failed to launch after login: incorrect package(expected: %s, actual: %s)", appPkgName, currentAppPkg)
	}
	testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}
