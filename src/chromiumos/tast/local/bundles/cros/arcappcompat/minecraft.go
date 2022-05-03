// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
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

const (
	signOutTestCaseTimeoutForMinecraft = 4 * time.Minute
)

// clamshellLaunchForMinecraft launches Minecraft in clamshell mode.
var clamshellLaunchForMinecraft = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForMinecraft, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForMinecraft launches Minecraft in tablet mode.
var touchviewLaunchForMinecraft = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForMinecraft, Timeout: testutil.LaunchTestCaseTimeout},
}

// clamshellAppSpecificTestsForMinecraft are placed here.
var clamshellAppSpecificTestsForMinecraft = []testutil.TestCase{
	{Name: "Clamshell: Signout app", Fn: signOutOfMinecraft, Timeout: signOutTestCaseTimeoutForMinecraft},
}

// touchviewAppSpecificTestsForMinecraft are placed here.
var touchviewAppSpecificTestsForMinecraft = []testutil.TestCase{
	{Name: "Touchview: Signout app", Fn: signOutOfMinecraft, Timeout: signOutTestCaseTimeoutForMinecraft},
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
				LaunchTests:      clamshellLaunchForMinecraft,
				CommonTests:      testutil.ClamshellSmokeTests,
				AppSpecificTests: clamshellAppSpecificTestsForMinecraft,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForMinecraft,
				CommonTests:      testutil.TouchviewSmokeTests,
				AppSpecificTests: touchviewAppSpecificTestsForMinecraft,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForMinecraft,
				CommonTests:      testutil.ClamshellSmokeTests,
				AppSpecificTests: clamshellAppSpecificTestsForMinecraft,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForMinecraft,
				CommonTests:      testutil.TouchviewSmokeTests,
				AppSpecificTests: touchviewAppSpecificTestsForMinecraft,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 30 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
		VarDeps: []string{"arcappcompat.Minecraft.emailid", "arcappcompat.Minecraft.password"},
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
		appPageClassName    = "android.widget.FrameLayout"
		enterEmailAddressID = "i0116"
		nextButtonText      = "Next"
		passwordID          = "i0118"
		signInText          = "Sign in"
		notNowID            = "android:id/autofill_save_no"
		// The inputs rendered by Minecraft are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 10
	)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	t, ok := arc.Type()
	if !ok {
		s.Fatal("Unable to determine arc type")
	}
	deviceMode := "clamshell"
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if tabletModeEnabled && t == arc.VM {
		deviceMode = "tablet"
		s.Logf("device %v mode", deviceMode)
		// For arc-vm devices and for tablet mode , wait for the existence of enterEmailAddress field and then click on it.
		if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
			s.Fatal("EnterEmailAddress doesn't exists: ", err)
		} else if err := enterEmailAddress.Click(ctx); err != nil {
			s.Fatal("Failed to click on enterEmailAddress: ", err)
		}
	} else {
		s.Logf("device %v mode", deviceMode)
		// Enter email id.
		if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
			s.Fatal("EnterEmailAddress doesn't exists: ", err)
		} else if err := enterEmailAddress.Click(ctx); err != nil {
			s.Fatal("Failed to click on enterEmailAddress: ", err)
		}

		// For arc-P devices, click on emailid text field until the emailid text field is focused.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if emailIDFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
				return errors.New("email text field not focused yet")
			} else if !emailIDFocused {
				enterEmailAddress.Click(ctx)
				return errors.New("email text field not focused yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
			s.Log("Failed to focus EmailId: ", err)
		}
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	emailID := s.RequiredVar("arcappcompat.Minecraft.emailid")
	if err := kb.Type(ctx, emailID); err != nil {
		s.Fatal("Failed to enter emailID: ", err)
	}
	s.Log("Entered EmailAddress")

	// Click on next button
	nextButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+nextButtonText))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Next Button doesn't exists: ", err)
		// Press enter key to click on next button.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Fatal("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	deviceMode = "clamshell"
	enterPassword := d.Object(ui.ID(passwordID))
	if tabletModeEnabled && t == arc.VM {
		deviceMode = "tablet"
		s.Logf("device %v mode", deviceMode)
		// For arc-vm devices and for tablet mode, wait for the existence of enterpassword field and then click on it.
		ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute)
		if err := uiauto.Combine("Find password field",
			ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.TextBlock([]string{"password"}).Nth(0).Below(uidetection.TextBlock([]string{"Enter", "password"}))),
			ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).Tap(uidetection.TextBlock([]string{"password"}).Nth(0).Below(uidetection.TextBlock([]string{"Enter", "password"}))),
			action.Sleep(waitForActiveInputTime),
		)(ctx); err != nil {
			s.Fatal("Failed to find password field: ", err)
		}
	} else {
		s.Logf("device %v mode", deviceMode)
		// Enter password.
		if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
			s.Log("EnterPassword doesn't exists: ", err)
		} else if err := enterPassword.Click(ctx); err != nil {
			s.Fatal("Failed to click on enterPassword: ", err)
		}

		// For arc-P devices, click on password text field until the password text field is focused.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
				return errors.New("password text field not focused yet")
			} else if !pwdFocused {
				enterPassword.Click(ctx)
				return errors.New("password text field not focused yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
			s.Fatal("Failed to focus password: ", err)
		}
	}

	password := s.RequiredVar("arcappcompat.Minecraft.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	if tabletModeEnabled && t == arc.VM {
		deviceMode = "tablet"
		s.Logf("device %v mode", deviceMode)
		// Press enter key to click on sign in button.
		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			s.Log("Failed to enter KEYCODE_ENTER: ", err)
		} else {
			s.Log("Entered KEYCODE_ENTER")
		}
	} else {
		// Click on Sign in button.
		signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signInText))
		if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Error("SignInButton doesn't exists: ", err)
		}

		// Click on signIn Button until not now button exist.
		signInButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signInText))
		notNowButton := d.Object(ui.ID(notNowID))
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := notNowButton.Exists(ctx); err != nil {
				signInButton.Click(ctx)
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
			s.Log("notNowButton doesn't exist: ", err)
		} else if err := notNowButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on notNowButton: ", err)
		}
	}
	ud := uidetection.NewDefault(tconn).WithTimeout(time.Minute)
	if err := uiauto.Combine("Check for Play in the home screen",
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.Word("Flay")),
		action.Sleep(waitForActiveInputTime),
	)(ctx); err != nil {
		s.Fatal("Failed to find Play: ", err)
	}
}

// signOutOfMinecraft verifies app is signed out.
func signOutOfMinecraft(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		// The inputs rendered by Minecraft are not immediately active after being clicked
		// so wait a moment for the engine to make the input active before interacting with it.
		waitForActiveInputTime = time.Second * 10
	)
	// Check if sign in page does exists.
	// This happens when app is already signed out when performing any of the common test cases.
	ud := uidetection.NewDefault(tconn)
	if err := uiauto.Combine("Check for sign in page",
		ud.WithTimeout(testutil.ShortUITimeout).WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.TextBlock(strings.Split("Sign In", " ")).First()),
	)(ctx); err == nil {
		s.Log("Sign in does exist and app is signed out already")
		return
	}

	ud = uidetection.NewDefault(tconn)
	if err := uiauto.Combine("Find switch accounts",
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.TextBlock([]string{"Suitch", "Accounts"})),
		ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).Tap(uidetection.TextBlock([]string{"Suitch", "Accounts"}).First()),
		action.Sleep(waitForActiveInputTime),
		action.IfSuccessThen(
			ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(uidetection.TextBlock(strings.Split("Sign Out", " ")).Below(uidetection.TextBlock(strings.Split("Do you want to sign out and suitch accounts?", " ")))),
			ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).Tap(uidetection.TextBlock(strings.Split("Sign Out", " ")).Below(uidetection.TextBlock(strings.Split("Do you want to sign out and suitch accounts?", " ")))),
		),
		action.Sleep(waitForActiveInputTime),
	)(ctx); err != nil {
		s.Error("Failed to find switch account / sign out: ", err)
	}
}
