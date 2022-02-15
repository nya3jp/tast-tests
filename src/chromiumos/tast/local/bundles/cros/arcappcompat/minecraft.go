// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForMinecraft launches Minecraft in clamshell mode.
var clamshellLaunchForMinecraft = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForMinecraft},
}

// touchviewLaunchForMinecraft launches Minecraft in tablet mode.
var touchviewLaunchForMinecraft = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForMinecraft},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Minecraft,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Minecraft that installs and checks Minecraft correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForMinecraft,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForMinecraft,
				CommonTests: testutil.TouchviewSmokeTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForMinecraft,
				CommonTests: testutil.ClamshellSmokeTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForMinecraft,
				CommonTests: testutil.TouchviewSmokeTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
	})
}

// Minecraft test uses library for opting into the playstore and installing app.
// Checks Minecraft correctly changes the window states in both clamshell and touchview mode.
func Minecraft(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.mojang.minecraftedu"
		appActivity = "com.mojang.minecraftpe.MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForMinecraft verify app and
// verify app reached main activity page of the app.
func launchAppForMinecraft(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		appPageClassName = "android.widget.FrameLayout"
	)
	closeAndRelaunchApp(ctx, s, tconn, a, d, appPkgName, appActivity)
	appPage := d.Object(ui.ClassName(appPageClassName), ui.PackageName(appPkgName), ui.Enabled(true))
	if err := appPage.Exists(ctx); err == nil {
		s.Log("App page does exist")
		// Wait for app page to load.
		if err := testing.Sleep(ctx, testutil.ShortUITimeout); err != nil {
			s.Fatal("Failed to sleep for welcome page to load: ", err)
		}
		// To select try a demo lesson option.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		}
		s.Log("Entered KEYCODE_TAB")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		}
		s.Log("Entered KEYCODE_ENTER")
	}

	if err := appPage.Exists(ctx); err == nil {
		s.Log("App Page does exist")
		// Wait for view terms page to appear.
		if err := testing.Sleep(ctx, testutil.DefaultUITimeout); err != nil {
			s.Fatal("Failed to sleep for view terms page to appear: ", err)
		}
		// To select accept term option.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		}
		s.Log("Entered KEYCODE_TAB")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		}
		s.Log("Entered KEYCODE_TAB")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		}
		s.Log("Entered KEYCODE_ENTER")
		// To select play option.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		}
		s.Log("Entered KEYCODE_TAB")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		}
		s.Log("Entered KEYCODE_TAB")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		}
		s.Log("Entered KEYCODE_ENTER")
	}

	if err := appPage.Exists(ctx); err == nil {
		s.Log("selectStartLesson does exist")
		// Wait for select start lesson page to appear.
		if err := testing.Sleep(ctx, testutil.DefaultUITimeout); err != nil {
			s.Fatal("Failed to sleep for select start lesson page: ", err)
		}
		// To select start lesson option.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		}
		s.Log("Entered KEYCODE_TAB")
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		}
		s.Log("Entered KEYCODE_ENTER")
	}

	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}

// closeAndRelaunchApp func close and relaunch the app to skip login page and reach try a demo lesson option page.
// TODO(b/222120246): Remove closeAndRelaunchApp func once the login credentials are received from dev rel team.
func closeAndRelaunchApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	if err := a.Command(ctx, "am", "force-stop", appPkgName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to stop app: ", err)
	}
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()
	// Launch the app.
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	s.Log("Closed and relaunch the app successfully")
}
