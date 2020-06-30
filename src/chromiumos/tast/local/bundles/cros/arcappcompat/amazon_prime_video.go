// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForAmazonPrimeVideo = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForAmazonPrimeVideo},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForAmazonPrimeVideo = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForAmazonPrimeVideo},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: testutil.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AmazonPrimeVideo,
		Desc:         "Functional test for AmazonPrimeVideo that installs the app also verifies it is logged in and that the main page is open, checks AmazonPrimeVideo correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password", "arcappcompat.AmazonPrimeVideo.username", "arcappcompat.AmazonPrimeVideo.password"},
	})
}

// AmazonPrimeVideo test uses library for opting into the playstore and installing app.
// Checks AmazonPrimeVideo correctly changes the window states in both clamshell and touchview mode.
func AmazonPrimeVideo(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.amazon.avod.thirdpartyclient"
		appActivity = ".LauncherActivity"
	)

	// Step up chrome on Chromebook.
	cr, tconn, a, d := testutil.SetUpDevice(ctx, s, appPkgName, appActivity)
	defer d.Close()

	// Ensure app launches before test cases.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app before test cases: ", err)
	}
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app before test cases: ", err)
	}

	testSet := s.Param().(testutil.TestParams)
	// Run the different test cases.
	for idx, test := range testSet.Tests {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			// Launch the app.
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			s.Log("App launched successfully")

			defer act.Stop(ctx, tconn)

			// Take screenshot on failure.
			defer func() {
				if s.HasError() {
					filename := fmt.Sprintf("screenshot-arcappcompat-failed-test-%d.png", idx)
					path := filepath.Join(s.OutDir(), filename)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
				}
			}()

			testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForAmazonPrimeVideo verifies AmazonPrimeVideo is logged in and
// verify AmazonPrimeVideo reached main activity page of the app.
func launchAppForAmazonPrimeVideo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		allowButtonText       = "ALLOW"
		textViewClassName     = "android.widget.TextView"
		myStuffText           = "My Stuff"
		enterEmailAddressID   = "ap_email"
		nextButtonDescription = "Next"
		passwordClassName     = "android.widget.EditText"
		passwordID            = "ap_password"
		passwordText          = "Amazon password"
		signInText            = "Sign-In"
		sendOTPText           = "Send OTP"
	)

	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterEmailAddress does not exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	username := s.RequiredVar("arcappcompat.AmazonPrimeVideo.username")
	if err := kb.Type(ctx, username); err != nil {
		s.Fatal("Failed to enter username: ", err)
	}
	s.Log("Entered username")

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterPassword does not exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	kbp, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kbp.Close()

	password := s.RequiredVar("arcappcompat.AmazonPrimeVideo.password")
	if err := kbp.Type(ctx, password); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}
	s.Log("Entered password")

	// Keep clicking signIn Button until myStuff icon exist in the home page.
	signInButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signInText))
	myStuffIcon := d.Object(ui.ClassName(textViewClassName), ui.Text(myStuffText))
	sendOTPButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(sendOTPText))

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := myStuffIcon.Exists(ctx); err != nil {
			signInButton.Click(ctx)
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: testutil.LongUITimeout}); err != nil {
		s.Error("MyStuffIcon doesn't exist: ", err)
		// Check for send OTP button
		if err := sendOTPButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Log("Send OTP Button doesn't exist")
		} else {
			s.Error("Send OTP Button does exist: ", err)
		}
	} else {
		s.Log("MyStuffIcon does exist ")
	}
	signOutAmazonPrimeVideo(ctx, s, a, d, appPkgName, appActivity)
}

// signOutAmazonPrimeVideo verifies app is signed out.
func signOutAmazonPrimeVideo(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		textViewClassName             = "android.widget.TextView"
		myStuffText                   = "My Stuff"
		settingsIconClassName         = "android.widget.ImageButton"
		settingsIconDescription       = "Settings"
		selectSignedInOptionClassName = "android.widget.TextView"
		selectSignedInOptionText      = "Signed in as Music Tester"
		signOutText                   = "SIGN OUT"
	)

	// Click on my stuff icon.
	myStuffIcon := d.Object(ui.ClassName(textViewClassName), ui.Text(myStuffText))
	if err := myStuffIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("MyStuffIcon doesn't exist: ", err)
	} else if err := myStuffIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click MyStuffIcon: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ClassName(settingsIconClassName), ui.DescriptionContains(settingsIconDescription))
	if err := settingsIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SettingsIcon doesn't exist: ", err)
	} else if err := settingsIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on settingsIcon: ", err)
	}
	// Select signed in option as music tester.
	selectSignedInOption := d.Object(ui.ClassName(selectSignedInOptionClassName), ui.Text(selectSignedInOptionText))
	if err := selectSignedInOption.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SelectSignedInOption doesn't exist: ", err)
	} else if err := selectSignedInOption.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectSignedInOption: ", err)
	}
	// Click on sign out button.
	signOutButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signOutText))
	if err := signOutButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Error("SignOutButton doesn't exist: ", err)
	} else if err := signOutButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on signOutButton: ", err)
	}
}
