// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForSkype launches Skype in clamshell mode.
var clamshellLaunchForSkype = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSkype, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForSkype launches Skype in tablet mode.
var touchviewLaunchForSkype = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSkype, Timeout: testutil.LaunchTestCaseTimeout},
}

// clamshellAppSpecificTestsForSkype are placed here.
var clamshellAppSpecificTestsForSkype = []testutil.TestCase{
	{Name: "Clamshell: Signout app", Fn: signOutOfSkype, Timeout: testutil.SignoutTestCaseTimeout},
}

// touchviewAppSpecificTestsForSkype are placed here.
var touchviewAppSpecificTestsForSkype = []testutil.TestCase{
	{Name: "Touchview: Signout app", Fn: signOutOfSkype, Timeout: testutil.SignoutTestCaseTimeout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Skype,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Skype that installs the app also verifies it is logged in and that the main page is open, checks Skype correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForSkype,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForSkype,
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
				LaunchTests:      touchviewLaunchForSkype,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForSkype,
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
				LaunchTests:      clamshellLaunchForSkype,
				CommonTests:      testutil.ClamshellCommonTests,
				AppSpecificTests: clamshellAppSpecificTestsForSkype,
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
				LaunchTests:      touchviewLaunchForSkype,
				CommonTests:      testutil.TouchviewCommonTests,
				AppSpecificTests: touchviewAppSpecificTestsForSkype,
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
				LaunchTests:      clamshellLaunchForSkype,
				ReleaseTests:     testutil.ClamshellReleaseTests,
				AppSpecificTests: clamshellAppSpecificTestsForSkype,
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
				LaunchTests:      touchviewLaunchForSkype,
				ReleaseTests:     testutil.TouchviewReleaseTests,
				AppSpecificTests: touchviewAppSpecificTestsForSkype,
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
				LaunchTests:      clamshellLaunchForSkype,
				ReleaseTests:     testutil.ClamshellReleaseTests,
				AppSpecificTests: clamshellAppSpecificTestsForSkype,
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
				LaunchTests:      touchviewLaunchForSkype,
				ReleaseTests:     testutil.TouchviewReleaseTests,
				AppSpecificTests: touchviewAppSpecificTestsForSkype,
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
		VarDeps: []string{"arcappcompat.Skype.emailid", "arcappcompat.Skype.password"},
	})
}

// Skype test uses library for opting into the playstore and installing app.
// Checks Skype correctly changes the window states in both clamshell and touchview mode.
func Skype(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.skype.raider"
		appActivity = ".Main"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForSkype verifies Skype is logged in and
// verify Skype reached main activity page of the app.
func launchAppForSkype(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText             = "ALLOW"
		continueButtonDes           = "Continue"
		letsGoDes                   = "Let's go"
		enterEmailAddressID         = "i0116"
		finishButtonDes             = "Finish"
		nextButtonText              = "Next"
		notNowID                    = "android:id/autofill_save_no"
		passwordID                  = "i0118"
		signInClassName             = "android.widget.Button"
		signInText                  = "Sign in"
		signInOrCreateDes           = "Sign in or create"
		syncContactsButtonDes       = "Sync contacts"
		whileUsingThisAppButtonText = "WHILE USING THE APP"
		mediumUITimeout             = 30 * time.Second
	)
	// Click on letsGo button.
	letsGoButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+letsGoDes))
	if err := letsGoButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("letsGoButton doesn't exists: ", err)
	} else if err := letsGoButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on letsGoButton: ", err)
	}

	// Click on sign in button.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signInText))
	appVer, err := testutil.GetAppVersion(ctx, s, a, d, appPkgName)
	if err != nil {
		s.Log("Failed to find app version and skipped login: ", err)
		return
	}
	if strings.Compare(appVer, "8.80.0.137") >= 0 {
		// Click on sign in button.
		signInButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+signInOrCreateDes))
	}
	if err := signInButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Fatal("signInButton doesn't exists: ", err)
	}

	// Press KEYCODE_TAB until login button is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if loginBtnFocused, err := signInButton.IsFocused(ctx); err != nil {
			return errors.New("login button not focused yet")
		} else if !loginBtnFocused {
			d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0)
			return errors.New("login button not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: mediumUITimeout}); err != nil {
		s.Log("Failed to focus login button: ", err)
	}

	// click on signin button until allow button exists.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := allowButton.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("Allow button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}
	testutil.LoginToApp(ctx, s, tconn, a, d, appPkgName, appActivity)

	// Click on continue button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+continueButtonDes))
	if err := continueButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("Continue Button doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Click on Sync contacts button.
	syncContactsButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+syncContactsButtonDes))
	if err := syncContactsButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("syncContactsButton doesn't exist: ", err)
	} else if err := syncContactsButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on syncContactsButton: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Click on continue Button until allow button exist.
	continueButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+continueButtonDes))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := allowButton.Exists(ctx); err != nil {
			continueButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("allowButton doesn't exist: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Click on finish button.
	finishButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Description(finishButtonDes))
	if err := finishButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("finishButton doesn't exist: ", err)
	} else if err := finishButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on finishButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}

// signOutOfSkype verifies app is signed out.
func signOutOfSkype(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		continueButtonDes    = "Continue"
		finishButtonDes      = "Finish"
		imageButtonClassName = "android.widget.ImageButton"
		closeIconDes         = "Close main menus"
		profileClassName     = "android.widget.Button"
		profileDes           = "My info"
		hamburgerIconDes     = "Menu"
		skipText             = "Skip"
		signOutID            = "com.skype.raider:id/drawer_signout"
		signOutDes           = "Sign out"
		yesText              = "YES"
	)
	appVer, err := testutil.GetAppVersion(ctx, s, a, d, appPkgName)
	if err != nil {
		s.Log("Failed to find app version and skipped login: ", err)
		return
	}
	// Click on continue Button until allow button exist.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+continueButtonDes))
	skipButton := d.Object(ui.TextMatches("(?i)" + skipText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := skipButton.Exists(ctx); err != nil {
			continueButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.DefaultUITimeout}); err != nil {
		s.Log("skipButton doesn't exist: ", err)
	}

	// Click on finish button.
	finishButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Description(finishButtonDes))
	if err := finishButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("finishButton doesn't exist: ", err)
	} else if err := finishButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on finishButton: ", err)
	}
	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)

	// Check for profileIcon.
	profileIcon := d.Object(ui.ClassName(profileClassName), ui.DescriptionMatches("(?i)"+profileDes), ui.Index(1))
	if strings.Compare(appVer, "8.80.0.137") <= 0 {
		s.Log("Current app version is less than 8.80.0.137")
		profileIcon = d.Object(ui.ClassName(imageButtonClassName), ui.DescriptionMatches("(?i)"+hamburgerIconDes))
	}
	if err := profileIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("profileIcon doesn't exists and skipped logout: ", err)
		return
	}

	// Click on profile icon until sign out button exist.
	signOutOfSkype := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+signOutDes))
	if strings.Compare(appVer, "8.80.0.137") > 0 {
		testutil.ClickUntilButtonExists(ctx, s, tconn, a, d, profileIcon, signOutOfSkype)
	} else if err := profileIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on profileIcon: ", err)
	}

	// Click on sign out of Skype.
	if strings.Compare(appVer, "8.80.0.137") <= 0 {
		signOutOfSkype = d.Object(ui.ID(signOutID))
	}
	if err := signOutOfSkype.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("signOutOfSkype doesn't exist: ", err)
	} else if err := signOutOfSkype.Click(ctx); err != nil {
		s.Fatal("Failed to click on signOutOfSkype: ", err)
	}

	// Click on yes button.
	yesButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+yesText))
	if err := yesButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("yesButton doesn't exists: ", err)
	} else if err := yesButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on yesButton: ", err)
	}
}
