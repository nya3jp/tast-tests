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
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ClamshellTests are placed here.
var clamshellTestsForFacebook = []testutil.TestSuite{
	{Name: "Launch app in Clamshell", Fn: launchAppForFacebook},
	{Name: "Clamshell: Fullscreen app", Fn: testutil.ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: testutil.ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: reOpenWindowForFacebookAndSignout},
}

// TouchviewTests are placed here.
var touchviewTestsForFacebook = []testutil.TestSuite{
	{Name: "Launch app in Touchview", Fn: launchAppForFacebook},
	{Name: "Touchview: Minimise and Restore", Fn: testutil.MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: reOpenWindowForFacebookAndSignout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Facebook,
		Desc:         "Functional test for Facebook that installs the app also verifies it is logged in and that the main page is open, checks Facebook correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForFacebook,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForFacebook,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}, {
			Name: "vm",
			Val: testutil.TestParams{
				TabletMode: false,
				Tests:      clamshellTestsForFacebook,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               pre.AppCompatBooted,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				TabletMode: true,
				Tests:      touchviewTestsForFacebook,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               pre.AppCompatBootedInTabletMode,
		}},
		Timeout: 10 * time.Minute,
		Vars: []string{"arcappcompat.username", "arcappcompat.password",
			"arcappcompat.Facebook.emailid", "arcappcompat.Facebook.password"},
	})
}

// Facebook test uses library for opting into the playstore and installing app.
// Checks Facebook correctly changes the window states in both clamshell and touchview mode.
func Facebook(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.facebook.katana"
		appActivity = ".LoginActivity"
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

// launchAppForFacebook verifies Facebook is logged in and
// verify Facebook reached main activity page of the app.
func launchAppForFacebook(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		textClassName          = "android.widget.EditText"
		userNameDes            = "Username"
		passwordDes            = "Password"
		noThanksText           = "NO THANKS"
		allowDes               = "Allow"
		allowText              = "ALLOW"
		okText                 = "OK"
		hamburgerIconClassName = "android.view.View"
	)
	var indexNum = 5

	if currentAppPkg := testutil.CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("Failed to launch the app: ", currentAppPkg)
	}
	s.Log("App is launched successfully in launchAppForFacebook")

	// Enter email address.
	FacebookEmailID := s.RequiredVar("arcappcompat.Facebook.emailid")
	enterEmailAddress := d.Object(ui.ClassName(textClassName), ui.Description(userNameDes))
	if err := enterEmailAddress.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterEmailAddress doesn't exist: ", err)
	} else if err := enterEmailAddress.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterEmailAddress: ", err)
	} else if enterEmailAddress.SetText(ctx, FacebookEmailID); err != nil {
		s.Fatal("Failed to enterEmailAddress: ", err)
	}

	// Enter Password.
	FacebookPassword := s.RequiredVar("arcappcompat.Facebook.password")
	enterPassword := d.Object(ui.ClassName(textClassName), ui.Description(passwordDes))
	if err := enterPassword.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("EnterPassword doesn't exist: ", err)
	} else if err := enterPassword.Click(ctx); err != nil {
		s.Fatal("Failed to click on enterPassword: ", err)
	} else if enterPassword.SetText(ctx, FacebookPassword); err != nil {
		s.Fatal("Failed to enterPassword: ", err)
	}

	// Click on login button.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to press enter button: ", err)
	}

	// Click on no thanks button.
	clickOnNoThanksButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(noThanksText))
	if err := clickOnNoThanksButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnNoThanksButton doesn't exist: ", err)
	} else if err := clickOnNoThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnNoThanksButton: ", err)
	}

	// Click on allow button.
	clickOnAllowButton := d.Object(ui.Description(allowDes))
	if err := clickOnAllowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := clickOnAllowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAllowButton: ", err)
	}

	// Click on allow button again.
	clickOnAllowButton = d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(allowText))
	if err := clickOnAllowButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := clickOnAllowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnAllowButton: ", err)
	}

	// Click on ok button to turn on device location.
	clickOnOkButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.Text(okText))
	if err := clickOnOkButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("clickOnOkButton doesn't exist: ", err)
	} else if err := clickOnOkButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on clickOnOkButton: ", err)
	}

	// Check for hambuger Icon.
	hambugerIcon := d.Object(ui.ClassName(hamburgerIconClassName), ui.Index(indexNum))
	if err := hambugerIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("hambugerIcon doesn't exist: ", err)
	}

}

// reOpenWindowForFacebookAndSignout Test "close and relaunch the app" and signout from the app.
func reOpenWindowForFacebookAndSignout(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	s.Log("Close the app")
	if err := a.Command(ctx, "am", "force-stop", appPkgName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to close the app: ", err)
	}
	testutil.DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	// Create an app activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	s.Log("Created new app activity")

	defer act.Close()
	// ReLaunch the activity.
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	s.Log("App relaunched successfully")

	signOutOfFacebook(ctx, s, a, d, appPkgName, appActivity)
}

// signOutOfFacebook verifies app is signed out.
func signOutOfFacebook(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		viewClassName   = "android.view.View"
		scrollClassName = "androidx.recyclerview.widget.RecyclerView"
		logoutClassName = "android.view.ViewGroup"
		logoutDes       = "Log Out, Button 1 of 1"
	)
	var indexNum = 5

	// Click on menu icon.
	menuIcon := d.Object(ui.ClassName(viewClassName), ui.Index(indexNum))
	if err := menuIcon.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("menuIcon doesn't exist: ", err)
	} else if err := menuIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click on menuIcon: ", err)
	}

	// Scroll until logout is visible.
	scrollLayout := d.Object(ui.ClassName(scrollClassName), ui.Scrollable(true))
	if err := scrollLayout.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("LogOutOfFacebook doesn't exist: ", err)
	}

	logOutOfFacebook := d.Object(ui.ClassName(logoutClassName), ui.Description(logoutDes))
	scrollLayout.ScrollTo(ctx, logOutOfFacebook)
	if err := logOutOfFacebook.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("logOutOfFacebook doesn't exist: ", err)
	}

	// Click on log out of Facebook.
	if err := logOutOfFacebook.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Error("LogOutOfFacebook doesn't exist: ", err)
	} else if err := logOutOfFacebook.Click(ctx); err != nil {
		s.Fatal("Failed to click on logOutOfFacebook: ", err)
	}
}
