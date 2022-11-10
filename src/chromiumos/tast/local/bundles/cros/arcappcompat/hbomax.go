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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForHbomax launches Hbomax in clamshell mode.
var clamshellLaunchForHbomax = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForHbomax, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForHbomax launches Hbomax in tablet mode.
var touchviewLaunchForHbomax = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForHbomax, Timeout: testutil.LaunchTestCaseTimeout},
}

// clamshellAppSpecificTestsForHbomax are placed here.
var clamshellAppSpecificTestsForHbomax = []testutil.TestCase{
	{Name: "Clamshell: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

// touchviewAppSpecificTestsForHbomax are placed here.
var touchviewAppSpecificTestsForHbomax = []testutil.TestCase{
	{Name: "Touchview: Video Playback", Fn: testutil.TouchAndPlayVideo},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Hbomax,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Hbomax that install, launch the app and check that the main page is open, also checks Hbomax correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		// TODO(b/254308076): Remove the skipped models once the solution is found for the issue.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("eve"), hwdep.SkipOnModel("nocturne")),
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForHbomax,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForHbomax,
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
				LaunchTests:      touchviewLaunchForHbomax,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForHbomax,
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
				LaunchTests:      clamshellLaunchForHbomax,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForHbomax,
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
				LaunchTests:      touchviewLaunchForHbomax,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForHbomax,
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
				LaunchTests:      clamshellLaunchForHbomax,
				ReleaseTests:     testutil.ClamshellReleaseTests,
				AppSpecificTests: clamshellAppSpecificTestsForHbomax,
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
				LaunchTests:      touchviewLaunchForHbomax,
				ReleaseTests:     testutil.TouchviewReleaseTests,
				AppSpecificTests: touchviewAppSpecificTestsForHbomax,
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
				LaunchTests:      clamshellLaunchForHbomax,
				ReleaseTests:     testutil.ClamshellReleaseTests,
				AppSpecificTests: clamshellAppSpecificTestsForHbomax,
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
				LaunchTests:      touchviewLaunchForHbomax,
				ReleaseTests:     testutil.TouchviewReleaseTests,
				AppSpecificTests: touchviewAppSpecificTestsForHbomax,
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
		VarDeps: []string{"arcappcompat.Hbomax.emailid", "arcappcompat.Hbomax.password"},
	})
}

// Hbomax test uses library for opting into the playstore and installing app.
// Checks Hbomax correctly changes the window states in both clamshell and touchview mode.
func Hbomax(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.hbo.hbonow"
		appActivity = ".MainActivity"
	)

	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForHbomax verifies Hbomax is launched and
// verify Hbomax reached main activity page of the app.
func launchAppForHbomax(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		signupBtnID      = "HeroNavigateButton"
		enterEmailAddrID = "EmailTextInput"
		notNowID         = "android:id/autofill_save_no"
		pwdID            = "PasswordTextInput"
		profileID        = "AvatarContentPressableContainer"
		profileBtnID     = "ProfileButton"
		profileNameDes   = "appcompat"
		signInBtnID      = "SignInButton"

		waitForActiveInputTime = time.Second * 10
	)
	// Click on sign up button.
	signupBtn := d.Object(ui.ID(signupBtnID))
	if err := signupBtn.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("signupBtn doesn't exist and skipped login")
		return
	} else if err := signupBtn.Click(ctx); err != nil {
		s.Fatal("Failed to click on signupBtn: ", err)
	}

	// Click on sign in to your account button.
	signInBtn := uidetection.TextBlock([]string{"SIGN", "IN", "TO", "YOUR", "ACCOUNT"})
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	if err := uiauto.Combine("Check for sign in button",
		ud.WaitUntilExists(signInBtn),
		action.Sleep(waitForActiveInputTime),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).LeftClick(signInBtn),
		action.Sleep(testutil.DefaultUITimeout),
	)(ctx); err != nil {
		s.Log("Failed to find signInBtn: ", err)
	}

	// Enter email id.
	enterEmailAddr := d.Object(ui.ID(enterEmailAddrID))
	if err := enterEmailAddr.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("EnterEmailAddress doesn't exists: ", err)
	} else if err := enterEmailAddr.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}
	// Click on emailid text field until the emailid text field is focused.
	testutil.ClickUntilFocused(ctx, s, tconn, a, d, enterEmailAddr)

	emailAddress := s.RequiredVar("arcappcompat.Hbomax.emailid")
	if err := enterEmailAddr.SetText(ctx, emailAddress); err != nil {
		s.Fatal("Failed to enter EmailAddress: ", err)
	}
	s.Log("Entered EmailAddress")

	// Enter password.
	enterPassword := d.Object(ui.ID(pwdID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("EnterPassword doesn't exists: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	// Click on password text field until the password text field is focused.
	testutil.ClickUntilFocused(ctx, s, tconn, a, d, enterPassword)

	password := s.RequiredVar("arcappcompat.Hbomax.password")
	if err := enterPassword.SetText(ctx, password); err != nil {
		s.Fatal("Failed to enter enterPassword: ", err)
	}
	s.Log("Entered password")

	signInButton := d.Object(ui.ID(signInBtnID))
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("signInButton doesn't exist")
	}

	// Click on sign in button until not now button exists.
	notNowButton := d.Object(ui.ID(notNowID))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, signInButton, notNowButton)
	testutil.HandleSavePasswordToGoogle(ctx, s, tconn, a, d, appPkgName)

	// Click on profile name until home icon exists.
	profileName := d.Object(ui.ID(profileID), ui.DescriptionStartsWith(profileNameDes))
	profileBtn := d.Object(ui.ID(profileBtnID))
	testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, profileName, profileBtn)

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}
