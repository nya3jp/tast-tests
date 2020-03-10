// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
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
			Pre:               arc.BootedAppCompat(),
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               arc.VMBootedInTabletModeAppCompat(),
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// AmazonPrimeVideo test uses library for opting into the playstore and installing app.
// Checks AmazonPrimeVideo correctly changes the window states in both clamshell and touchview mode.
func AmazonPrimeVideo(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.amazon.avod.thirdpartyclient"
		appActivity = "com.amazon.identity.auth.device.AuthPortalUIActivity"

		openButtonRegex = "Open|OPEN"
	)

	// Step up chrome on Chromebook.
	cr, tconn, a, d := testutil.SetUpDevice(ctx, s, appPkgName, appActivity)

	s.Log("Launch the app")
	// Click on open button.
	openButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches(openButtonRegex))
	if err := openButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("Open Button doesn't exist: ", err)
	} else if err := openButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on openButton: ", err)
	}

	testSet := s.Param().(testutil.TestParams)
	// Run the different test cases.
	for idx, test := range testSet.Tests {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			defer func() {
				if s.HasError() {
					path := fmt.Sprintf("%s/screenshot-arcappcompat-failed-test-%d.png", s.OutDir(), idx)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
				}
			}()
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
}

// launchAppForAmazonPrimeVideo verifies AmazonPrimeVideo is logged in and
// verify AmazonPrimeVideo reached main activity page of the app.
func launchAppForAmazonPrimeVideo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
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

	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_A: ", err)
	}
	s.Log("Entered KEYCODE_A")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_M, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_M: ", err)
	}
	s.Log("Entered KEYCODE_M")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_A: ", err)
	}
	s.Log("Entered KEYCODE_A")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_Z, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_Z: ", err)
	}
	s.Log("Entered KEYCODE_Z")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_O: ", err)
	}
	s.Log("Entered KEYCODE_O")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_N, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_N: ", err)
	}
	s.Log("Entered KEYCODE_N")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_MINUS, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_MINUS: ", err)
	}
	s.Log("Entered KEYCODE_MINUS")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_P, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_P: ", err)
	}
	s.Log("Entered KEYCODE_P")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_A: ", err)
	}
	s.Log("Entered KEYCODE_A")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_R, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_R: ", err)
	}
	s.Log("Entered KEYCODE_R")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_T, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_T: ", err)
	}
	s.Log("Entered KEYCODE_T")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_N, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_N: ", err)
	}
	s.Log("Entered KEYCODE_N")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_E, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_E: ", err)
	}
	s.Log("Entered KEYCODE_E")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_R, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_R: ", err)
	}
	s.Log("Entered KEYCODE_R")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_S, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_S: ", err)
	}
	s.Log("Entered KEYCODE_S")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_MINUS, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_MINUS: ", err)
	}
	s.Log("Entered KEYCODE_MINUS")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_G: ", err)
	}
	s.Log("Entered KEYCODE_G")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_O: ", err)
	}
	s.Log("Entered KEYCODE_O")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Fatal("Failed toenter KEYCODE_O: ", err)
	}
	s.Log("Entered KEYCODE_O")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, 0); err != nil {
		s.Fatal("Failed toenter KEYCODE_G: ", err)
	}
	s.Log("Entered KEYCODE_G")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_L, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_L: ", err)
	}
	s.Log("Entered KEYCODE_L")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_E, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_E: ", err)
	}
	s.Log("Entered KEYCODE_E")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_2, ui.META_SHIFT_ON); err != nil {
		s.Fatal("Failed to enter KEYCODE_2: ", err)
	}
	s.Log("Entered KEYCODE_2")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_A: ", err)
	}
	s.Log("Entered KEYCODE_A")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_M, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_M: ", err)
	}
	s.Log("Entered KEYCODE_M")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_A: ", err)
	}
	s.Log("Entered KEYCODE_A")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_Z, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_Z: ", err)
	}
	s.Log("Entered KEYCODE_Z")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_O: ", err)
	}
	s.Log("Entered KEYCODE_O")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_N, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_N: ", err)
	}
	s.Log("Entered KEYCODE_N")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_PERIOD, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_PERIOD: ", err)
	}
	s.Log("Entered KEYCODE_PERIOD")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_C, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_C: ", err)
	}
	s.Log("Entered KEYCODE_C")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_O: ", err)
	}
	s.Log("Entered KEYCODE_O")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_M, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_M: ", err)
	}
	s.Log("Entered KEYCODE_M")

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	if err := enterPassword.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal("EnterPassword does not exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	}

	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, ui.META_SHIFT_ON); err != nil {
		s.Fatal("Failed to enter KEYCODE_G: ", err)
	}
	s.Log("Entered KEYCODE_G")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_O: ", err)
	}
	s.Log("Entered KEYCODE_O")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_O: ", err)
	}
	s.Log("Entered KEYCODE_O")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_G: ", err)
	}
	s.Log("Entered KEYCODE_G")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_L, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_L: ", err)
	}
	s.Log("Entered KEYCODE_L")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_E, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_E: ", err)
	}
	s.Log("Entered KEYCODE_E")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_T, ui.META_SHIFT_ON); err != nil {
		s.Fatal("Failed to enter KEYCODE_T: ", err)
	}
	s.Log("Entered KEYCODE_T")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_E, 0); err != nil {
		s.Fatal("DFailed to enter KEYCODE_E: ", err)
	}
	s.Log("Entered KEYCODE_E")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_S, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_S: ", err)
	}
	s.Log("Entered KEYCODE_S")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_T, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_T: ", err)
	}
	s.Log("Entered KEYCODE_T")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_I, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_I: ", err)
	}
	s.Log("Entered KEYCODE_I")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_N, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_N: ", err)
	}
	s.Log("Entered KEYCODE_N")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_G: ", err)
	}
	s.Log("Entered KEYCODE_G")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_0, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_0: ", err)
	}
	s.Log("Entered KEYCODE_0")

	if err := d.PressKeyCode(ctx, ui.KEYCODE_1, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_1: ", err)
	}
	s.Log("Entered KEYCODE_1")

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
func signOutAmazonPrimeVideo(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
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
