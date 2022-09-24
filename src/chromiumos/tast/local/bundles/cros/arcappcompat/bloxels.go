// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForBloxels launches Bloxels in clamshell mode.
var clamshellLaunchForBloxels = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForBloxels, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForBloxels launches Bloxels in tablet mode.
var touchviewLaunchForBloxels = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForBloxels, Timeout: testutil.LaunchTestCaseTimeout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Bloxels,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Bloxels that installs the app also verifies if the main page is open, checks Bloxels correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"bloxels-got-it-button.png", "bloxels-play-now-button.png"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForBloxels,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForBloxels,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForBloxels,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForBloxels,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "clamshell_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForBloxels,
				TopAppTests: testutil.ClamshellTopAppTests,
			},
			ExtraAttr:         []string{"appcompat_top_apps"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForBloxels,
				TopAppTests: testutil.TouchviewTopAppTests,
			},
			ExtraAttr:         []string{"appcompat_top_apps"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForBloxels,
				TopAppTests: testutil.ClamshellTopAppTests,
			},
			ExtraAttr:         []string{"appcompat_top_apps"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode_top_apps",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForBloxels,
				TopAppTests: testutil.TouchviewTopAppTests,
			},
			ExtraAttr:         []string{"appcompat_top_apps"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 20 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
	})
}

// Bloxels test uses library for opting into the playstore and installing app.
// Checks Bloxels correctly changes the window states in both clamshell and touchview mode.
func Bloxels(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.projectpixelpress.BloxelsEDU"
		appActivity = "com.unity3d.player.UnityPlayerActivity"
	)

	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForBloxels verifies Bloxels is logged in and
// verify Bloxels reached main activity page of the app.
func launchAppForBloxels(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		allowText              = "ALLOW"
		continueText           = "CONTINUE"
		playWord               = "PLAY"
		playNowText            = "Play Now"
		whileUsingAppText      = "WHILE USING THE APP"
		waitForActiveInputTime = time.Second * 10
	)

	// Click on allow button to access photos and media.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowText))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, allowButton, launchVerifier)

	// Click on allow button to take picures and video.
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, allowButton, launchVerifier)

	// Click on while using app button to take picures and video.
	whileUsingAppButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+whileUsingAppText))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, whileUsingAppButton, launchVerifier)

	// Click on got it button.
	gotItButton := uidetection.CustomIcon(s.DataPath("bloxels-got-it-button.png"))
	ud := uidetection.NewDefault(tconn).WithTimeout(testutil.ShortUITimeout).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for got it button",
		ud.WaitUntilExists(gotItButton),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(gotItButton),
	)(ctx); err != nil {
		s.Log("Got it button does not exist: ", err)
	}

	// Click on play button.
	playButton := uidetection.Word(playWord)
	ud = uidetection.NewDefault(tconn).WithTimeout(testutil.DefaultUITimeout).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for play button",
		ud.WaitUntilExists(playButton),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(playButton),
	)(ctx); err != nil {
		s.Log("Play button does not exist: ", err)
	}

	// Click on play now button.
	playNowButton := uidetection.CustomIcon(s.DataPath("bloxels-play-now-button.png"))
	ud = uidetection.NewDefault(tconn).WithTimeout(testutil.ShortUITimeout).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for play now button",
		ud.WaitUntilExists(playNowButton),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(playNowButton),
	)(ctx); err != nil {
		s.Log("Play now button does not exist: ", err)
	}

	// Relaunch the app to skip ads.
	testutil.CloseAndRelaunchApp(ctx, s, tconn, a, d, appPkgName, appActivity)

	// Check for launch verifier.
	launchVerifier = d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
