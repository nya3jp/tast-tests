// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForAmazonPrimeVideo = []testutil.TestSuite{
	{"Launch app in Clamshell", launchAppForAmazonPrimeVideo},
	{"Clamshell: Fullscreen app", testutil.ClamshellFullscreenApp},
	{"Clamshell: Minimise and Restore", testutil.MinimizeRestoreApp},
	{"Clamshell: Resize window", testutil.ClamshellResizeWindow},
	{"Clamshell: Reopen app", testutil.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForAmazonPrimeVideo = []testutil.TestSuite{
	{"Launch app in Touchview", launchAppForAmazonPrimeVideo},
	{"Touchview: Minimise and Restore", testutil.MinimizeRestoreApp},
	{"Touchview: Reopen app", testutil.ReOpenWindow},
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
				false,
				clamshellTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedAppCompat(),
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				true,
				touchviewTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				false,
				clamshellTestsForAmazonPrimeVideo,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				true,
				touchviewTestsForAmazonPrimeVideo,
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

	// Setup Chrome.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()
	s.Log("Enable showing ANRs")
	if err := a.Command(ctx, "settings", "put", "secure", "anr_show_background", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable showing ANRs: ", err)
	}
	s.Log("Enable crash dialog")
	if err := a.Command(ctx, "settings", "put", "secure", "show_first_crash_dialog_dev_option", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to enable crash dialog: ", err)
	}

	s.Log("Installing app")
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	s.Log("Launch the app")
	// Click on open button.
	openButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches(openButtonRegex))
	must(openButton.WaitForExists(ctx, testutil.LongUITimeout))
	// Open button exist and click.
	must(openButton.Click(ctx))

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
func launchAppForAmazonPrimeVideo(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	const (
		allowButtonText       = "ALLOW"
		textViewClassName     = "android.widget.TextView"
		myStuffText           = "My Stuff"
		enterEmailAddressID   = "ap_email"
		nextButtonDescription = "Next"
		passwordClassName     = "android.widget.EditText"
		passwordID            = "ap_password"
		passwordText          = "Amazon password"
		emailAddress          = "amazon-partners-google@amazon.com"
		passWord              = "GoogleTesting01"
		signInText            = "Sign-In"
		sendOTPText           = "Send OTP"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}
	// Click on allow button to access your photos, media and files.
	allowButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else {
		must(allowButton.Click(ctx))
	}

	// Enter email id.
	enterEmailAddress := d.Object(ui.ID(enterEmailAddressID))
	must(enterEmailAddress.WaitForExists(ctx, testutil.LongUITimeout))

	must(enterEmailAddress.Click(ctx))
	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_A: ", err)
	} else {
		s.Log("Entered KEYCODE_A")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_M, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_M: ", err)
	} else {
		s.Log("Entered KEYCODE_M")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_A: ", err)
	} else {
		s.Log("Entered KEYCODE_A")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_Z, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_Z: ", err)
	} else {
		s.Log("Entered KEYCODE_Z")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_O: ", err)
	} else {
		s.Log("Entered KEYCODE_O")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_N, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_N: ", err)
	} else {
		s.Log("Entered KEYCODE_N")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_MINUS, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_MINUS: ", err)
	} else {
		s.Log("Entered KEYCODE_MINUS")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_P, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_P: ", err)
	} else {
		s.Log("Entered KEYCODE_P")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_A: ", err)
	} else {
		s.Log("Entered KEYCODE_A")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_R, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_R: ", err)
	} else {
		s.Log("Entered KEYCODE_R")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_T, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_T: ", err)
	} else {
		s.Log("Entered KEYCODE_T")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_N, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_N: ", err)
	} else {
		s.Log("Entered KEYCODE_N")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_E, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_E: ", err)
	} else {
		s.Log("Entered KEYCODE_E")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_R, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_R: ", err)
	} else {
		s.Log("Entered KEYCODE_R")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_S, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_S: ", err)
	} else {
		s.Log("Entered KEYCODE_S")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_MINUS, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_MINUS: ", err)
	} else {
		s.Log("Entered KEYCODE_MINUS")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_G: ", err)
	} else {
		s.Log("Entered KEYCODE_G")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_O: ", err)
	} else {
		s.Log("Entered KEYCODE_O")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_O: ", err)
	} else {
		s.Log("Entered KEYCODE_O")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_G: ", err)
	} else {
		s.Log("Entered KEYCODE_G")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_L, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_L: ", err)
	} else {
		s.Log("Entered KEYCODE_L")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_E, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_E: ", err)
	} else {
		s.Log("Entered KEYCODE_E")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_2, ui.META_SHIFT_ON); err != nil {
		s.Log("Doesn't enter KEYCODE_2: ", err)
	} else {
		s.Log("Entered KEYCODE_2")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_A: ", err)
	} else {
		s.Log("Entered KEYCODE_A")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_M, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_M: ", err)
	} else {
		s.Log("Entered KEYCODE_M")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_A, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_A: ", err)
	} else {
		s.Log("Entered KEYCODE_A")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_Z, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_Z: ", err)
	} else {
		s.Log("Entered KEYCODE_Z")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_O: ", err)
	} else {
		s.Log("Entered KEYCODE_O")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_N, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_N: ", err)
	} else {
		s.Log("Entered KEYCODE_N")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_PERIOD, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_PERIOD: ", err)
	} else {
		s.Log("Entered KEYCODE_PERIOD")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_C, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_C: ", err)
	} else {
		s.Log("Entered KEYCODE_C")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_O: ", err)
	} else {
		s.Log("Entered KEYCODE_O")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_M, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_M: ", err)
	} else {
		s.Log("Entered KEYCODE_M")
	}

	// Enter password.
	enterPassword := d.Object(ui.ID(passwordID))
	must(enterPassword.WaitForExists(ctx, testutil.LongUITimeout))

	must(enterPassword.Click(ctx))
	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, ui.META_SHIFT_ON); err != nil {
		s.Log("Doesn't enter KEYCODE_G: ", err)
	} else {
		s.Log("Entered KEYCODE_G")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_O: ", err)
	} else {
		s.Log("Entered KEYCODE_O")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_O, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_O: ", err)
	} else {
		s.Log("Entered KEYCODE_O")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_G: ", err)
	} else {
		s.Log("Entered KEYCODE_G")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_L, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_L: ", err)
	} else {
		s.Log("Entered KEYCODE_L")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_E, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_E: ", err)
	} else {
		s.Log("Entered KEYCODE_E")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_T, ui.META_SHIFT_ON); err != nil {
		s.Log("Doesn't enter KEYCODE_T: ", err)
	} else {
		s.Log("Entered KEYCODE_T")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_E, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_E: ", err)
	} else {
		s.Log("Entered KEYCODE_E")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_S, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_S: ", err)
	} else {
		s.Log("Entered KEYCODE_S")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_T, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_T: ", err)
	} else {
		s.Log("Entered KEYCODE_T")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_I, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_I: ", err)
	} else {
		s.Log("Entered KEYCODE_I")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_N, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_N: ", err)
	} else {
		s.Log("Entered KEYCODE_N")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_G, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_G: ", err)
	} else {
		s.Log("Entered KEYCODE_G")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_0, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_0: ", err)
	} else {
		s.Log("Entered KEYCODE_0")
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_1, 0); err != nil {
		s.Log("Doesn't enter KEYCODE_1: ", err)
	} else {
		s.Log("Entered KEYCODE_1")
	}
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
		s.Log("myStuffIcon doesn't exist: ", err)
		// Check for send OTP button
		if err := sendOTPButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
			s.Log("Send OTP Button doesn't exist")
		} else {
			s.Error("Send OTP Button does exist: ", err)
		}
	} else {
		s.Log("myStuffIcon does exist ")
	}
	defer signOutAmazonPrimeVideo(ctx, s, a, d, appPkgName, appActivity)
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
	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}
	// Click on my stuff icon.
	myStuffIcon := d.Object(ui.ClassName(textViewClassName), ui.Text(myStuffText))
	if err := myStuffIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("myStuffIcon doesn't exist: ", err)
		// Launch the activity.
		if err := a.Command(ctx, "am", "start", appPkgName+"/"+appActivity).Run(); err != nil {
			s.Log("Failed starting app: ", err)
		}
	} else {
		s.Log("myStuffIcon does exist")
	}
	if err := myStuffIcon.Click(ctx); err != nil {
		s.Log("myStuffIcon doesn't clicked: ", err)
	}

	// Click on settings icon.
	settingsIcon := d.Object(ui.ClassName(settingsIconClassName), ui.DescriptionContains(settingsIconDescription))
	if err := settingsIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("settingsIcon doesn't exist: ", err)
	} else {
		must(settingsIcon.Click(ctx))
	}
	// Select signed in option as music tester.
	selectSignedInOption := d.Object(ui.ClassName(selectSignedInOptionClassName), ui.Text(selectSignedInOptionText))
	if err := selectSignedInOption.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("selectSignedInOption doesn't exist: ", err)
	} else {
		must(selectSignedInOption.Click(ctx))
	}
	// Click on sign out button.
	signOutButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(signOutText))
	if err := signOutButton.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Log("signOutButton doesn't exist: ", err)
	} else {
		must(signOutButton.Click(ctx))
	}
}
