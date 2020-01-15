// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleCalendar,
		Desc:         "Functional test for Google Calendar that installs the app also verifies it is logged in and that the main page is open",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"arcappcompat.username", "arcappcompat.password"},
	})
}

// GoogleCalendar test uses library for opting into the playstore and installing app.
// Launch the app from playstore.
// Verify app is logged in.
// Verify app reached main activity page of the app.
func GoogleCalendar(ctx context.Context, s *testing.State) {
	const (
		appPkgName = "com.google.android.calendar"

		addButtonClassName       = "android.widget.ImageButton"
		addButtonDescription     = "Create new event"
		allowButtonText          = "ALLOW"
		androidButtonClassName   = "android.widget.Button"
		gotItButtonText          = "Got it"
		hamburgerIconClassName   = "android.widget.ImageButton"
		hamburgerIconDescription = "Show Calendar List and Settings drawer"
		nextIconID               = "com.google.android.calendar:id/next_arrow_touch"
		openButtonClassName      = "android.widget.Button"
		openButtonText           = "Open"
		userNameID               = "com.google.android.calendar:id/tile"

		defaultUITimeout = 20 * time.Second
		longUITimeout    = 5 * time.Minute
	)
	username := s.RequiredVar("arcappcompat.username")
	password := s.RequiredVar("arcappcompat.password")

	// Setup Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.ExtraArgs("--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()
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
	if err := playstore.InstallApp(ctx, a, d, appPkgName); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	must := func(err error) {
		if err != nil {
			s.Fatal("Error occurred: ", err) // NOLINT: arc/ui returns loggable errors
		}
	}

	// Launch Google Calendar app.
	// Click on open button.
	openButton := d.Object(ui.ClassName(openButtonClassName), ui.Text(openButtonText))
	must(openButton.WaitForExists(ctx, longUITimeout))
	// Open button exists and click
	must(openButton.Click(ctx))

	// Verify login and homescreen of Google Calendar app.
	// Check for next icon.
	nextIcon := d.Object(ui.ID(nextIconID))
	for count := 0; count < 4; count++ {
		if err := nextIcon.WaitForExists(ctx, defaultUITimeout); err != nil {
			s.Log("Next Icon does not exists: ", err)
		} else {
			nextIcon.Click(ctx)
		}
	}

	// Check for got it button.
	gotItButton := d.Object(ui.ClassName(androidButtonClassName), ui.Text(gotItButtonText))
	if err := gotItButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("GotIt Button doesn't exists: ", err)
	} else {
		gotItButton.Click(ctx)
	}

	// Click on allow button to access your calendar.
	allowButton := d.Object(ui.ClassName(androidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else {
		allowButton.Click(ctx)
	}

	// Click on allow button to access your contacts.
	if err := allowButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exists: ", err)
	} else {
		allowButton.Click(ctx)
	}

	// Click on hambugerIcon in home page of the app.
	hamburgerIcon := d.Object(ui.ClassName(hamburgerIconClassName), ui.DescriptionContains(hamburgerIconDescription))
	must(hamburgerIcon.WaitForExists(ctx, defaultUITimeout))
	must(hamburgerIcon.Click(ctx))

	// Check app is logged in with username.
	userName := d.Object(ui.ID(userNameID))
	must(userName.WaitForExists(ctx, longUITimeout))

	// Click on press back.
	must(d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0))

	// Check for add icon in home page.
	addIcon := d.Object(ui.ClassName(addButtonClassName), ui.DescriptionContains(addButtonDescription))
	must(addIcon.WaitForExists(ctx, longUITimeout))

}
