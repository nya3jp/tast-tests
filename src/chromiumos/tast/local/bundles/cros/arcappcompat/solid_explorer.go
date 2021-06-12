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

// clamshellLaunchForSolidExplorer launches SolidExplorer in clamshell mode.
var clamshellLaunchForSolidExplorer = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForSolidExplorer},
}

// touchviewLaunchForSolidExplorer launches SolidExplorer in tablet mode.
var touchviewLaunchForSolidExplorer = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForSolidExplorer},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SolidExplorer,
		Desc:         "Functional test for SolidExplorer that installs the app also verifies it is logged in and that the main page is open, checks SolidExplorer correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForSolidExplorer,
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
				Tests:      touchviewLaunchForSolidExplorer,
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
				Tests:      clamshellLaunchForSolidExplorer,
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
				Tests:      touchviewLaunchForSolidExplorer,
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

// SolidExplorer test uses library for opting into the playstore and installing app.
// Checks SolidExplorer correctly changes the window states in both clamshell and touchview mode.
func SolidExplorer(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "pl.solidexplorer2"
		appActivity = "pl.solidexplorer.SolidExplorer"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForSolidExplorer verifies SolidExplorer is logged in and
// verify SolidExplorer reached main activity page of the app.
func launchAppForSolidExplorer(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		skipID    = "pl.solidexplorer2:id/btn_skip"
		licenceID = "pl.solidexplorer2:id/cb_license"
		gotItText = "GOT IT"
		doneID    = "pl.solidexplorer2:id/btn_next"
		allowText = "ALLOW"
		OkText    = "OK"
	)

	// Click on skip button.
	skipButton := d.Object(ui.ID(skipID))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Skip button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skip button: ", err)
	}

	// Click on licence button.
	licenceButton := d.Object(ui.ID(licenceID))
	if err := licenceButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Licence button doesn't exist: ", err)
	} else if err := licenceButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on licence button: ", err)
	}

	// Click on got it button.
	gotItButton := d.Object(ui.Text(gotItText))
	if err := gotItButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Got It button doesn't exist: ", err)
	} else if err := gotItButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Got It button: ", err)
	}

	// Click on done button.
	doneButton := d.Object(ui.ID(doneID))
	if err := doneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Done button doesn't exist: ", err)
	} else if err := doneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on done button: ", err)
	}

	// Click on Allow button until OK button exist.
	allowButton := d.Object(ui.Text(allowText))
	OKButton := d.Object(ui.Text(OkText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := OKButton.Exists(ctx); err != nil {
			s.Log(" Click on allow button until OK button exist")
			allowButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Log("OK button doesn't exist: ", err)
	}

	// Click on OK button.
	if err := OKButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Ok button doesn't exist: ", err)
	} else if err := OKButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Ok button: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
