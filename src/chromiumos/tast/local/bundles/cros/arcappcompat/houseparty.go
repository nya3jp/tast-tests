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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// clamshellLaunchForHouseparty launches Houseparty in clamshell mode.
var clamshellLaunchForHouseparty = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForHouseparty},
}

// touchviewLaunchForHouseparty launches Houseparty in tablet mode.
var touchviewLaunchForHouseparty = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForHouseparty},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     Houseparty,
		Desc:     "Functional test for Houseparty that installs the app also verifies it is logged in and that the main page is open, checks Houseparty correctly changes the window state in both clamshell and touchview mode",
		Contacts: []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		// Disabled this test as Houseparty android app is not available anymore and it is migrated to PWA app.
		//Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForHouseparty,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForHouseparty,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:      clamshellLaunchForHouseparty,
				CommonTest: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:      touchviewLaunchForHouseparty,
				CommonTest: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Houseparty.emailid", "arcappcompat.Houseparty.password"},
	})
}

// Houseparty test uses library for opting into the playstore and installing app.
// Checks Houseparty correctly changes the window states in both clamshell and touchview mode.
func Houseparty(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.herzick.houseparty"
		appActivity = "com.lifeonair.houseparty.ui.routing.RoutingActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForHouseparty verifies Houseparty is logged in and
// verify Houseparty reached main activity page of the app.
func launchAppForHouseparty(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText        = "ALLOW"
		continueText           = "CONTINUE"
		editTextClassName      = "android.widget.EditText"
		enterEmailAddressText  = "Email or Username"
		iHaveAnAccountButtonID = "com.herzick.houseparty:id/splash_login_button"
		nextID                 = "com.herzick.houseparty:id/frame_layout"
		notNowID               = "android:id/autofill_save_no"
		passwordText           = "Password"
		homeID                 = "com.herzick.houseparty:id/house_activity_top_buttons_inbox_button"
	)

	// Click on I have an account button.
	clickOnIHavaAnAccountButton := d.Object(ui.ID(iHaveAnAccountButtonID))
	if err := clickOnIHavaAnAccountButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("clickOnIHavaAnAccountButton doesn't exists: ", err)
	} else if err := clickOnIHavaAnAccountButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnIHavaAnAccountButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ClassName(editTextClassName), ui.Text(enterEmailAddressText))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exists: ", err)
	}

	// Click on email address text field until the email address text field is focused.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if emailAddressFocused, err := enterEmailAddress.IsFocused(ctx); err != nil {
			return errors.New("emailAddress text field not focused yet")
		} else if !emailAddressFocused {
			enterEmailAddress.Click(ctx)
			return errors.New("emailAddress text field not focused yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus emailAddress: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	emailAddress := s.RequiredVar("arcappcompat.Houseparty.emailid")
	if err := kb.Type(ctx, emailAddress); err != nil {
		s.Fatal("Failed to enter emailAddress: ", err)
	}
	s.Log("Entered EmailAddress")

	// Enter password.
	enterPassword := d.Object(ui.ClassName(editTextClassName), ui.Text(passwordText))
	if err := enterPassword.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("EnterPassword doesn't exists: ", err)
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
	}, &testing.PollOptions{Timeout: testutil.ShortUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.Houseparty.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on next button.
	clickOnNextButton := d.Object(ui.ID(nextID))
	if err := clickOnNextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNextButton doesn't exist: ", err)
	} else if err := clickOnNextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNextButton: ", err)
	}

	// Click on notNow button.
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}
	// Click on allow button to access your files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Click on allow button to access your videos.
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Check for homeIcon on homePage.
	homeIcon := d.Object(ui.ID(homeID))
	if err := homeIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Error("homeIcon doesn't exists: ", err)
	}

}
