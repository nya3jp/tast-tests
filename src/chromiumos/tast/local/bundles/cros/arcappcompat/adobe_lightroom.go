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

// clamshellLaunchForAdobeLightroom launches AdobeLightroom in clamshell mode.
var clamshellLaunchForAdobeLightroom = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForAdobeLightroom},
}

// touchviewLaunchForAdobeLightroom launches AdobeLightroom in tablet mode.
var touchviewLaunchForAdobeLightroom = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForAdobeLightroom},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdobeLightroom,
		Desc:         "Functional test for AdobeLightroom that installs the app also verifies it is logged in and that the main page is open, checks AdobeLightroom correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForAdobeLightroom,
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
				Tests:      touchviewLaunchForAdobeLightroom,
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
				Tests:      clamshellLaunchForAdobeLightroom,
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
				Tests:      touchviewLaunchForAdobeLightroom,
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

// AdobeLightroom test uses library for opting into the playstore and installing app.
// Checks AdobeLightroom correctly changes the window states in both clamshell and touchview mode.
func AdobeLightroom(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.adobe.lrmobile"
		appActivity = ".StorageCheckActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForAdobeLightroom verifies AdobeLightroom is logged in and
// verify AdobeLightroom reached main activity page of the app.
func launchAppForAdobeLightroom(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		skipID         = "com.adobe.lrmobile:id/gotoLastPage"
		googleID       = "com.adobe.lrmobile:id/google"
		emailAddressID = "com.google.android.gms:id/container"
	)
	var loginWithGoogleIndex int

	// Click on skip button.
	skipButton := d.Object(ui.ID(skipID))
	if err := skipButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("skip button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip in button: ", err)
	}

	// Check for google button.
	googleButton := d.Object(ui.ID(googleID))
	if err := googleButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("Google button doesn't exist: ", err)
	}

	// Click on Google button until email address exists.
	emailAddress := d.Object(ui.ID(emailAddressID), ui.Index(loginWithGoogleIndex))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := emailAddress.Exists(ctx); err != nil {
			googleButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("emailAddress doesn't exists: ", err)
	}

	// Click on email address.
	if err := emailAddress.Exists(ctx); err != nil {
		s.Log("EmailAddress doesn't exist: ", err)
	} else if err := emailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on EmailAddress: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
