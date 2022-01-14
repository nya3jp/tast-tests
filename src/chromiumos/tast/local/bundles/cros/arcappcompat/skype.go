// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForSkype launches Skype in clamshell mode.
var clamshellLaunchForSkype = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForSkype},
}

// touchviewLaunchForSkype launches Skype in tablet mode.
var touchviewLaunchForSkype = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForSkype},
}

// clamshellAppSpecificTestsForSkype are placed here.
var clamshellAppSpecificTestsForSkype = []testutil.TestCase{
	{Name: "Clamshell: Signout app", Fn: signOutOfSkype},
}

// touchviewAppSpecificTestsForSkype are placed here.
var touchviewAppSpecificTestsForSkype = []testutil.TestCase{
	{Name: "Touchview: Signout app", Fn: signOutOfSkype},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Skype,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for Skype that installs the app also verifies it is logged in and that the main page is open, checks Skype correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_release"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForSkype,
				CommonTests:      testutil.ClamshellSmokeTests,
				AppSpecificTests: clamshellAppSpecificTestsForSkype,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForSkype,
				CommonTests:      testutil.TouchviewSmokeTests,
				AppSpecificTests: touchviewAppSpecificTestsForSkype,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests:      clamshellLaunchForSkype,
				CommonTests:      testutil.ClamshellSmokeTests,
				AppSpecificTests: clamshellAppSpecificTestsForSkype,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests:      touchviewLaunchForSkype,
				CommonTests:      testutil.TouchviewSmokeTests,
				AppSpecificTests: touchviewAppSpecificTestsForSkype,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 10 * time.Minute,
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
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
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

		// Click on emailid text field until the emailid text field is focused.
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

	emailID := s.RequiredVar("arcappcompat.Skype.emailid")
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
		// Enter password.
		if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
			s.Log("EnterPassword doesn't exists: ", err)
		} else if err := enterPassword.Click(ctx); err != nil {
			s.Log("Failed to click on enterPassword: ", err)
		}
	} else {
		s.Logf("device %v mode", deviceMode)
		// Enter password.
		if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
			s.Log("EnterPassword doesn't exists: ", err)
		} else if err := enterPassword.Click(ctx); err != nil {
			s.Fatal("Failed to click on enterPassword: ", err)
		}

		// Click on password text field until the password text field is focused.
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

	password := s.RequiredVar("arcappcompat.Skype.password")
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
		signInButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+signInText))
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
		closeIconClassName = "android.widget.ImageButton"
		closeIconDes       = "Close main menus"
		profileClassName   = "android.widget.Button"
		profileDes         = "My info"
		signOutDes         = "Sign out"
		yesText            = "YES"
	)
	// Check for profileIcon.
	profileIcon := d.Object(ui.ClassName(profileClassName), ui.DescriptionMatches("(?i)"+profileDes))
	if err := profileIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("profileIcon doesn't exists and skipped logout: ", err)
		return
	}
	// Click on profile icon.
	if err := profileIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on profileIcon: ", err)
	}

	// Click on sign out of Skype.
	signOutOfSkype := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+signOutDes))
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
