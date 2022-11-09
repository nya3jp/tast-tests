// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForInstagram launches Instagram in clamshell mode.
var clamshellLaunchForInstagram = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForInstagram},
}

// touchviewLaunchForInstagram launches Instagram in tablet mode.
var touchviewLaunchForInstagram = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForInstagram},
}

// clamshellAppSpecificTestsForInstagram are placed here.
var clamshellAppSpecificTestsForInstagram = []testutil.TestCase{
	{Name: "Clamshell: Signout app", Fn: signOutOfInstagram, Timeout: testutil.SignoutTestCaseTimeout},
}

// touchviewAppSpecificTestsForInstagram are placed here.
var touchviewAppSpecificTestsForinstagram = []testutil.TestCase{
	{Name: "Touchview: Signout app", Fn: signOutOfInstagram, Timeout: testutil.SignoutTestCaseTimeout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Instagram,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Instagram that installs the app also verifies it is logged in and that the main page is open, checks Instagram correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForInstagram,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForInstagram,
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
				LaunchTests:      touchviewLaunchForInstagram,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForinstagram,
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
				LaunchTests:      clamshellLaunchForInstagram,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForInstagram,
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
				LaunchTests:      touchviewLaunchForInstagram,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForinstagram,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "clamshell_mode_release",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForInstagram,
				ReleaseTests:     testutil.ClamshellReleaseTests,
				AppSpecificTests: clamshellAppSpecificTestsForInstagram,
			},
			ExtraAttr:         []string{"appcompat_release"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode_release",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForInstagram,
				ReleaseTests:     testutil.TouchviewReleaseTests,
				AppSpecificTests: touchviewAppSpecificTestsForinstagram,
			},
			ExtraAttr:         []string{"appcompat_release"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode_release",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForInstagram,
				ReleaseTests:     testutil.ClamshellReleaseTests,
				AppSpecificTests: clamshellAppSpecificTestsForInstagram,
			},
			ExtraAttr:         []string{"appcompat_release"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode_release",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForInstagram,
				ReleaseTests:     testutil.TouchviewReleaseTests,
				AppSpecificTests: touchviewAppSpecificTestsForinstagram,
			},
			ExtraAttr:         []string{"appcompat_release"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 30 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.Instagram.username", "arcappcompat.Instagram.password"},
	})
}

// Instagram test uses library for opting into the playstore and installing app.
// Checks Instagram correctly changes the window states in both clamshell and touchview mode.
func Instagram(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.instagram.android"
		appActivity = ".activity.MainTabActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForInstagram verifies Instagram is logged in and
// verify Instagram reached main activity page of the app.
func launchAppForInstagram(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const waitForActiveInputTime = time.Second * 10

	// Click on Log in button.
	loginBtn := uidetection.TextBlock([]string{"Log", "in"})
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for login button",
		ud.WaitUntilExists(loginBtn),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(loginBtn),
		action.Sleep(testutil.DefaultUITimeout),
	)(ctx); err != nil {
		s.Log("Failed to find login button: ", err)
	}

	// Press tab key three times to click on enter phone number, email or username.
	for count := 0; count < 3; count++ {
		if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
			s.Log("Failed to enter KEYCODE_TAB: ", err)
		} else {
			s.Log("Entered KEYCODE_TAB")
			d.WaitForIdle(ctx, testutil.DefaultUITimeout)
		}
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.Instagram.username")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Press tab to click on password field
	d.WaitForIdle(ctx, testutil.DefaultUITimeout)
	if err := d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0); err != nil {
		s.Log("Failed to enter KEYCODE_TAB: ", err)
	} else {
		s.Log("Entered KEYCODE_TAB")
		d.WaitForIdle(ctx, testutil.DefaultUITimeout)
	}

	password := s.RequiredVar("arcappcompat.Instagram.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Press enter button to click on sign in button.
	d.WaitForIdle(ctx, testutil.DefaultUITimeout)
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	} else {
		s.Log("Entered KEYCODE_ENTER")
		d.WaitForIdle(ctx, testutil.DefaultUITimeout)
	}

	// Click on dimiss button to save password.
	testutil.HandleSavePasswordToGoogle(ctx, s, tconn, a, d, appPkgName)

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}

// signOutOfInstagram verifies app is signed out.
func signOutOfInstagram(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		avatarIconID      = "com.instagram.android:id/tab_avatar"
		optionsIconDes    = "Options"
		settingsDes       = "Settings"
		textViewClassName = "android.widget.TextView"
		logoutText        = "Log out"
		scrollID          = "com.instagram.android:id/recycler_view"
		notNowText        = "Not Now"
	)

	// Check for avatarIcon.
	avatarIcon := d.Object(ui.ID(avatarIconID))
	if err := avatarIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("avatarIcon doesn't exist and skipped logout: ", err)
		return
	} else if err := avatarIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on avatarIcon: ", err)
	}

	// Click on hambergerIcon.
	hamburgerIcon := d.Object(ui.DescriptionMatches("(?i)" + optionsIconDes))
	if err := hamburgerIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("hamburgerIcon doesn't exist and skipped logout: ", err)
		return
	} else if err := hamburgerIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on hamburgerIcon: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.DescriptionMatches("(?i)" + settingsDes))
	if err := settingsIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("settingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Scroll until logout is visible.
	scrollLayout := d.Object(ui.ID(scrollID), ui.Scrollable(true))
	deviceMode := "clamshell"
	if tabletModeEnabled {
		deviceMode = "tablet"
		scrollLayout = d.Object(ui.ID(scrollID))
	}
	s.Logf("device %v mode", deviceMode)
	if err := scrollLayout.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("scrollLayout doesn't exist and skipped logout: ", err)
		return
	}

	logOutOfInstagram := d.Object(ui.ClassName(textViewClassName), ui.TextMatches("(?i)"+logoutText))
	scrollLayout.ScrollTo(ctx, logOutOfInstagram)
	if err := logOutOfInstagram.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("logOutOfInstagram doesn't exist: ", err)
	} else if err := logOutOfInstagram.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfInstagram: ", err)
	}
	// Click on not now button to save login info.
	notNowButton := d.Object(ui.TextMatches("(?i)" + notNowText))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Click on log out button again.
	logoutBtn := d.Object(ui.TextMatches("(?i)" + logoutText))
	if err := logoutBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("logoutBtn doesn't exist: ", err)
	} else if err := logoutBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on logoutBtn: ", err)
	}

	// Click on not now button to save login info again.
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}
}
