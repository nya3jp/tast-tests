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
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForPaypal launches Paypal in clamshell mode.
var clamshellLaunchForPaypal = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForPaypal},
}

// touchviewLaunchForPaypal launches Paypal in tablet mode.
var touchviewLaunchForPaypal = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForPaypal},
}

// clamshellAppSpecificTestsForPaypal are placed here.
var clamshellAppSpecificTestsForPaypal = []testutil.TestSuite{
	{Name: "Clamshell: Signout app", Fn: signOutOfPaypal},
}

// touchviewAppSpecificTestsForPaypal are placed here.
var touchviewAppSpecificTestsForPaypal = []testutil.TestSuite{
	{Name: "Touchview: Signout app", Fn: signOutOfPaypal},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Paypal,
		Desc:         "Functional test for Paypal that installs the app also verifies it is logged in and that the main page is open, checks Paypal correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		// TODO (b/190409688) : Remove hwdep.SkipOnModel once the solution is found.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel(skipOnNoBackCameraModels...)),
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForPaypal,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForPaypal,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForPaypal,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForPaypal,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				Tests:           clamshellLaunchForPaypal,
				CommonTest:      testutil.ClamshellCommonTests,
				AppSpecificTest: clamshellAppSpecificTestsForPaypal,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				Tests:           touchviewLaunchForPaypal,
				CommonTest:      testutil.TouchviewCommonTests,
				AppSpecificTest: touchviewAppSpecificTestsForPaypal,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Paypal.emailid", "arcappcompat.Paypal.password"},
	})
}

// skipOnNoBackCameraModels is a list of models to be skipped from test runs.
var skipOnNoBackCameraModels = []string{
	"eve",
	"kevin",
	"caroline",
	"nasher360",
	"careena",
	"kasumi",
	"kasumi360",
	"treeya360",
	"treeya",
	"helios",
	"bluebird",
	"duffy",
	"sarien",
	"lazor",
	"elemi",
	"berknip",
}

// Paypal test uses library for opting into the playstore and installing app.
// Checks Paypal correctly changes the window states in both clamshell and touchview mode.
func Paypal(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.paypal.android.p2pmobile"
		appActivity = ".startup.activities.StartupActivity"
	)
	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForPaypal verifies Paypal is logged in and
// verify Paypal reached main activity page of the app.
func launchAppForPaypal(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		captchaText      = "A quick security check"
		enterEmailText   = "Email"
		loginButtonText  = "Log In"
		passwordText     = "Password"
		notNowID         = "android:id/autofill_save_no"
		notNowButtonText = "Not Now"
	)

	// Click on login button.
	clickOnLoginButton := d.Object(ui.TextMatches("(?i)" + loginButtonText))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("clickOnLoginButton doesn't exists: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.TextMatches("(?i)" + enterEmailText))
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
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus emailAddress: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	emailAddress := s.RequiredVar("arcappcompat.Paypal.emailid")
	if err := kb.Type(ctx, emailAddress); err != nil {
		s.Fatal("Failed to enter emailAddress: ", err)
	}
	s.Log("Entered EmailAddress")

	// Enter password.
	enterPassword := d.Object(ui.TextMatches("(?i)" + passwordText))
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
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Fatal("Failed to focus password: ", err)
	}

	password := s.RequiredVar("arcappcompat.Paypal.password")
	if err := kb.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Click on login button.
	clickOnLoginButton = d.Object(ui.TextMatches("(?i)" + loginButtonText))
	if err := clickOnLoginButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("clickOnLoginButton doesn't exists: ", err)
	} else if err := clickOnLoginButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnLoginButton: ", err)
	}

	// Click on notNow button.
	notNowButton := d.Object(ui.ID(notNowID))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exist: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	// Check for captcha.
	verifyCaptcha := d.Object(ui.TextMatches("(?i)" + captchaText))
	if err := verifyCaptcha.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("verifyCaptcha doesn't exist: ", err)
	} else {
		s.Log("Verify by reCaptcha exists")
		return
	}

	// Click on not now button.
	notNowButton = d.Object(ui.TextMatches("(?i)" + notNowButtonText))
	if err := notNowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("notNowButton doesn't exists: ", err)
	} else if err := notNowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on notNowButton: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}

}

// signOutOfPaypal verifies app is signed out.
func signOutOfPaypal(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		settingsIconClassName = "android.widget.ImageView"
		settingsIconDes       = "Settings"
		logOutOfPaypalText    = "Log Out"
		homeDes               = "Scan/Pay"
	)
	// Check for homeIcon.
	homeIcon := d.Object(ui.Description(homeDes))
	if err := homeIcon.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("homeIcon doesn't exists and skipped logout: ", err)
		return
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ClassName(settingsIconClassName), ui.Description(settingsIconDes))
	if err := settingsIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("SettingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}

	logOutOfPaypal := d.Object(ui.TextMatches("(?i)" + logOutOfPaypalText))
	if err := logOutOfPaypal.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("logOutOfPaypal doesn't exist: ", err)
	} else if err := logOutOfPaypal.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfPaypal: ", err)
	}

}
