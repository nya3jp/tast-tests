// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForMicrosoftOutlook launches MicrosoftOutlook in clamshell mode.
var clamshellLaunchForMicrosoftOutlook = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForMicrosoftOutlook},
}

// touchviewLaunchForMicrosoftOutlook launches MicrosoftOutlook in tablet mode.
var touchviewLaunchForMicrosoftOutlook = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForMicrosoftOutlook},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MicrosoftOutlook,
		Desc:         "Functional test for MicrosoftOutlook that installs the app also verifies it is logged in and that the main page is open, checks MicrosoftOutlook correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForMicrosoftOutlook,
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
				Tests:      touchviewLaunchForMicrosoftOutlook,
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
				Tests:      clamshellLaunchForMicrosoftOutlook,
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
				Tests:      touchviewLaunchForMicrosoftOutlook,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.MicrosoftOutlook.emailid", "arcappcompat.MicrosoftOutlook.password"},
	})
}

// MicrosoftOutlook test uses library for opting into the playstore and installing app.
// Checks MicrosoftOutlook correctly changes the window states in both clamshell and touchview mode.
func MicrosoftOutlook(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.microsoft.office.outlook"
		appActivity = ".MainActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForMicrosoftOutlook verifies MicrosoftOutlook is logged in and
// verify MicrosoftOutlook reached main activity page of the app.
func launchAppForMicrosoftOutlook(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		chromeAppPkgName       = "com.chrome.beta"
		chromeAppActivity      = "com.google.android.apps.chrome.Main"
		getStartedText         = "GET STARTED"
		enterEmailAddressID    = "com.microsoft.office.outlook:id/edit_text_email"
		continueText           = "CONTINUE"
		addAccountText         = "ADD ACCOUNT"
		acceptAndContinueText  = "Accept & continue"
		noThanksText           = "No thanks"
		nextText               = "Next"
		enterPasswordClassName = "android.widget.EditText"
		nextID                 = "passwordNext"
		neverText              = "Never"
		checkboxID             = "com.microsoft.office.outlook:id/sso_account_checkbox"
		mayBeLaterText         = "MAYBE LATER"
		composeID              = "com.microsoft.office.outlook:id/compose_fab"
		allowButtonText        = "Allow"
		scrollLayoutClassName  = "android.webkit.WebView"
		scrollLayoutText       = "Sign in - Google Accounts"
	)
	act, err := arc.NewActivity(a, appPkgName, appActivity)

	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop microsoft outlook app: ", err)
	}
	// Install chrome android app in order to make microsoft outlook work.
	// As mentioned in the help doc of outlook.
	installChromeAndroidApp(ctx, s, tconn, a, d, chromeAppPkgName, chromeAppActivity)

	// Relaunch the Microsoft outlook app.
	act, err = arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start Microsoft outlook: ", err)
	}

	loginHelperForChromeAndroidApp(ctx, s, tconn, a, d, chromeAppPkgName, chromeAppActivity)
	loginHelperForMicrosoftApp(ctx, s, tconn, a, d, appPkgName, appActivity)

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for homePageVerifier.
	homePageVerifier := d.Object(ui.ID(composeID))
	if err := homePageVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("homePageVerifier doesn't exists: ", err)
	}
}

// installChromeAndroidApp from playstore in order to launch microsoft outlook.
func installChromeAndroidApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, chromeAppPkgName, chromeAppActivity string) {
	const (
		getStartedText         = "GET STARTED"
		enterEmailAddressID    = "com.microsoft.office.outlook:id/edit_text_email"
		continueText           = "CONTINUE"
		addAccountText         = "ADD ACCOUNT"
		acceptAndContinueText  = "Accept & continue"
		noThanksText           = "No thanks"
		nextText               = "Next"
		enterPasswordClassName = "android.widget.EditText"
		nextID                 = "passwordNext"
		neverText              = "Never"
		layoutClassName        = "android.widget.FrameLayout"
	)
	cr := s.PreValue().(arc.PreData).Chrome
	act, err := arc.NewActivity(a, chromeAppPkgName, chromeAppActivity)

	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	// Launch the playstore.
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	// Install chrome android app.
	if err := playstore.InstallApp(ctx, a, d, chromeAppPkgName, 3); err != nil {
		s.Fatal("Failed to install chrome android app: ", err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
	}
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app: ", err)
	}
}

// loginHelperForChromeAndroidApp helps to grant permission for Microsoft Outlook app.
func loginHelperForChromeAndroidApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, chromeAppPkgName, chromeAppActivity string) {
	const (
		getStartedText         = "GET STARTED"
		enterEmailAddressID    = "com.microsoft.office.outlook:id/edit_text_email"
		continueText           = "CONTINUE"
		addAccountText         = "ADD ACCOUNT"
		acceptAndContinueText  = "Accept & continue"
		noThanksText           = "No thanks"
		nextText               = "Next"
		enterPasswordClassName = "android.widget.EditText"
		nextID                 = "passwordNext"
		neverText              = "Never"
		mayBeLaterText         = "MAYBE LATER"
		composeID              = "com.microsoft.office.outlook:id/compose_fab"
		allowButtonText        = "Allow"
		scrollLayoutClassName  = "android.webkit.WebView"
		scrollLayoutText       = "Sign in - Google Accounts"
		googleIconDes          = "Setup Google account."
	)
	act, err := arc.NewActivity(a, chromeAppPkgName, chromeAppActivity)
	// Click on getStarted button.
	getStartedButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+getStartedText))
	if err := getStartedButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("getStartedButton doesn't exists: ", err)
	} else if err := getStartedButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on getStartedButton: ", err)
	}

	// Click on addAccount button.
	addAccountButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+addAccountText))
	if err := addAccountButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("addAccountButton doesn't exists: ", err)
	} else if err := addAccountButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on addAccountButton: ", err)
	}

	// click on addAccountButton until enterEmailAddress exists.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
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
		s.Fatal("Failed to focus EmailId: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	emailAddress := s.RequiredVar("arcappcompat.MicrosoftOutlook.emailid")
	if err := enterEmailAddress.SetText(ctx, emailAddress); err != nil {
		s.Fatal("Failed to enter EmailAddress: ", err)
	}
	s.Log("Entered EmailAddress")

	// Click on continue button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Select google icon.
	googleIcon := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.DescriptionMatches("(?i)"+googleIconDes))
	if err := googleIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("googleIcon doesn't exists: ", err)
	} else if err := googleIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on googleIcon: ", err)
	}

	// Click on accept and continue button.
	acceptAndContinueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+acceptAndContinueText))
	if err := acceptAndContinueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("acceptAndContinueButton doesn't exists: ", err)
	} else if err := acceptAndContinueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on acceptAndContinueButton: ", err)
	}

	// Click on no thanks button.
	noThanksButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+noThanksText))
	if err := noThanksButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("noThanksButton doesn't exists: ", err)
	} else if err := noThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on noThanksButton: ", err)
	}

	// Click on next button.
	clickOnNextButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+nextText))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}

	// Enter password.
	enterPassword := d.Object(ui.ClassName(enterPasswordClassName))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterPassword doesn't exists: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	// Keep clicking password text field until the password text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if pwdFocused, err := enterPassword.IsFocused(ctx); err != nil {
			return errors.New("password text field not focused yet")
		} else if !pwdFocused {
			enterPassword.Click(ctx)
			return errors.New("password text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.MicrosoftOutlook.password")
	if err := enterPassword.SetText(ctx, password); err != nil {
		s.Fatal("Failed to enter enterPassword: ", err)
	}
	s.Log("Entered password")

	// Click on next button.
	clickOnNextButton = d.Object(ui.ID(nextID))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}

	// Click on never save button.
	clickOnNeverSaveButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+neverText))
	if err := clickOnNeverSaveButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNeverSaveButton doesn't exist: ", err)
	} else if err := clickOnNeverSaveButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNeverSaveButton: ", err)
	}
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop chrome android app: ", err)
	}
}

// loginHelperForMicrosoftApp helps to login to Microsoft Outlook app.
func loginHelperForMicrosoftApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		getStartedText         = "GET STARTED"
		enterEmailAddressID    = "com.microsoft.office.outlook:id/edit_text_email"
		continueText           = "CONTINUE"
		addAccountText         = "ADD ACCOUNT"
		acceptAndContinueText  = "Accept & continue"
		noThanksText           = "No thanks"
		nextText               = "Next"
		enterPasswordClassName = "android.widget.EditText"
		nextID                 = "passwordNext"
		neverText              = "Never"
		mayBeLaterText         = "MAYBE LATER"
		composeID              = "com.microsoft.office.outlook:id/compose_fab"
		allowButtonText        = "Allow"
		scrollLayoutClassName  = "android.webkit.WebView"
		scrollLayoutText       = "Sign in - Google Accounts"
	)

	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start Microsoft outlook: ", err)
	}
	// Click on getStarted button.
	getStartedButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+getStartedText))
	if err := getStartedButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("getStartedButton doesn't exists: ", err)
	} else if err := getStartedButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on getStartedButton: ", err)
	}

	// Click on addAccount button.
	addAccountButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+addAccountText))
	if err := addAccountButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("addAccountButton doesn't exists: ", err)
	} else if err := addAccountButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on addAccountButton: ", err)
	}

	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
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
		s.Fatal("Failed to focus EmailId: ", err)
	}

	emailAddress := s.RequiredVar("arcappcompat.MicrosoftOutlook.emailid")
	if err := enterEmailAddress.SetText(ctx, emailAddress); err != nil {
		s.Fatal("Failed to enter EmailAddress: ", err)
	}
	s.Log("Entered EmailAddress")

	// Click on continue button.
	continueButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+continueText))
	if err := continueButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("continueButton doesn't exists: ", err)
	} else if err := continueButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on continueButton: ", err)
	}

	// Click on next button.
	clickOnNextButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+nextText))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}

	// Enter password.
	enterPassword := d.Object(ui.ClassName(enterPasswordClassName))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("EnterPassword doesn't exists: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Log("Failed to click on enterPassword: ", err)
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
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.MicrosoftOutlook.password")
	if err := enterPassword.SetText(ctx, password); err != nil {
		s.Fatal("Failed to enter enterPassword: ", err)
	}
	s.Log("Entered password")

	// Click on next button.
	clickOnNextButton = d.Object(ui.ID(nextID))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}

	// Press on KEYCODE_TAB until allow button is focused.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if allowBtnFocused, err := allowButton.IsFocused(ctx); err != nil {
			return errors.New("allowButton not focused yet")
		} else if !allowBtnFocused {
			d.PressKeyCode(ctx, ui.KEYCODE_TAB, 0)
			return errors.New("allowButton not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Log("Failed to focus allowButton: ", err)
	}

	// Click on allow button.
	allowButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText), ui.Focused(true))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("allowButton doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Log("Failed to click on allowButton: ", err)
	}

	// Click on maybe later button.
	clickOnMayBeLaterButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+mayBeLaterText))
	if err := clickOnMayBeLaterButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnMayBeLaterButton doesn't exists: ", err)
	} else if err := clickOnMayBeLaterButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnMayBeLaterButton: ", err)
	}
}
