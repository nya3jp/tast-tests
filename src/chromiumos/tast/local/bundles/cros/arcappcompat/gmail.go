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
	"chromiumos/tast/local/bundles/cros/arcappcompat/reuse"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	androidButtonClassName = "android.widget.Button"

	defaultUITimeout = 20 * time.Second
	longUITimeout    = 5 * time.Minute
)

// testFunc represents the "test" function.
type testFunc func(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string)

// testSuite represents the  name of test, and the function to call.
type testSuite struct {
	name string
	fn   testFunc
}

// clamshellTestParams represents the collection of tests to run in tablet mode or clamshell mode.
type testParams struct {
	tabletMode bool
	tests      []testSuite
}

// ClamshellTests are placed here.
var clamshellTestsForGmail = []testSuite{
	{"Launch app in Clamshell", launchAppForGmail},
	{"Clamshell: Fullscreen app", reuse.ClamshellFullscreenApp},
	{"Clamshell: Minimise and Restore", reuse.MinimizeRestoreApp},
	{"Clamshell: Resize window", reuse.ClamshellResizeWindow},
	{"Clamshell: Reopen app", reuse.ReOpenWindow},
}

// TouchviewTests are placed here.
var touchviewTestsForGmail = []testSuite{
	{"Launch app in Touchview", launchAppForGmail},
	{"Touchview: Minimise and Restore", reuse.MinimizeRestoreApp},
	{"Touchview: Reopen app", reuse.ReOpenWindow},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gmail,
		Desc:         "Functional test for Gmail that installs the app also verifies it is logged in and that the main page is open, checks Gmail correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: testParams{
				false,
				clamshellTestsForGmail,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.BootedAppCompat(),
		}, {
			Name: "tablet_mode",
			Val: testParams{
				true,
				touchviewTestsForGmail,
			},
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			Pre:               arc.BootedInTabletModeAppCompat(),
		}, {
			Name: "vm",
			Val: testParams{
				false,
				clamshellTestsForGmail,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBootedAppCompat(),
		}, {
			Name: "vm_tablet_mode",
			Val: testParams{
				true,
				touchviewTestsForGmail,
			},
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			Pre:               arc.VMBootedInTabletModeAppCompat(),
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// Gmail test uses library for opting into the playstore and installing app.
// Checks Gmail correctly changes the window states in both clamshell and touchview mode.
func Gmail(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.gm"
		appActivity = "com.google.android.gm.ConversationListActivityGmail"

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
	openButton := d.Object(ui.ClassName(androidButtonClassName), ui.TextMatches(openButtonRegex))
	must(openButton.WaitForExists(ctx, longUITimeout))
	// Open button exists and click
	must(openButton.Click(ctx))

	testSet := s.Param().(testParams)
	// Run the different test cases.
	for idx, test := range testSet.tests {
		// Run subtests.
		s.Run(ctx, test.name, func(ctx context.Context, s *testing.State) {
			test.fn(ctx, s, tconn, a, d, appPkgName, appActivity)
			if err != nil {
				path := fmt.Sprintf("%s/screenshot-arcappcompat-failed-test-%d.png", s.OutDir(), idx)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}
				s.Errorf("%q test failed: %v", test.name, err)
			}
		})
	}
}

// launchAppForGmail verifies Gmail is logged in and
// verify Gmail reached main activity page of the app.
func launchAppForGmail(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {

	const (
		composeIconClassName                = "android.widget.ImageButton"
		composeIconDescription              = "Compose"
		gotItORtakeMeTOGmailButtonClassName = "android.widget.TextView"
		gotItButtonText                     = "GOT IT"
		takeMeTOGmailButtonText             = "TAKE ME TO GMAIL"
		userNameID                          = "com.google.android.gm:id/account_address"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	// Click on Got It button.
	GotItButton := d.Object(ui.ClassName(gotItORtakeMeTOGmailButtonClassName), ui.Text(gotItButtonText))
	if err := GotItButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("GotIt Button doesn't exist: ", err)
	} else {
		must(GotItButton.Click(ctx))
	}

	// Check app is logged in with username.
	userName := d.Object(ui.ID(userNameID))
	must(userName.WaitForExists(ctx, longUITimeout))

	// Click on TAKE ME TO GMAIL button.
	takeMeTOGmailButton := d.Object(ui.ClassName(gotItORtakeMeTOGmailButtonClassName), ui.Text(takeMeTOGmailButtonText))
	if err := takeMeTOGmailButton.WaitForExists(ctx, longUITimeout); err != nil {
		s.Log("TAKE ME TO GMAIL Button doesn't exist: ", err)
	} else {
		must(takeMeTOGmailButton.Click(ctx))
	}

	// Check for compose icon in home page.
	composeIcon := d.Object(ui.ClassName(composeIconClassName), ui.DescriptionContains(composeIconDescription))
	must(composeIcon.WaitForExists(ctx, longUITimeout))
}
