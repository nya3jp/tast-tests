// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappcompat

// Before running the test DUT(Chromebook) should be setup with following settings,
// DUT should be signed in with test account created using, rhea/
// Disable sync everything
// Disable apps sync
// Under developer options, enable adb debugging, always show crash dialog, show background ANRs,
// also enable show debug information in caption-title
import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestGoogleCalendarApp,
		Desc:         "Test to find, install, launch an app, also verify login and homepage of an app with UI automator",
		Contacts:     []string{"mthiyagarajan@chromium.org", "mthiyagarajan@google.com"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Timeout:      200 * time.Minute,
	})
}

// Search for Google Calendar app in the playstore.
// Install the app from playstore.
// Launch the app from playstore.
// Verify app is logged in.
// Verify app reached main activity page of the app.
// CleanUp, uninstall the app from the playstore.
func TestGoogleCalendarApp(ctx context.Context, s *testing.State) {
	const (
		appPkgName               = "com.google.android.calendar"
		allowButtonText          = "ALLOW"
		androidButtonClassName   = "android.widget.Button"
		addButtonClassName       = "android.widget.ImageButton"
		addButtonDescription     = "Create new event"
		gotItButtonText          = "Got it"
		hamburgerIconClassName   = "android.widget.ImageButton"
		hamburgerIconDescription = "Show Calendar List and Settings drawer"
		installButtonClassName   = "android.widget.Button"
		installButtonText        = "Install"
		nextIconID               = "com.google.android.calendar:id/next_arrow_touch"
		openButtonClassName      = "android.widget.Button"
		openButtonText           = "Open"
		playCardID               = "com.android.vending:id/play_card"
		playStorePkgName         = "com.android.vending"
		playStoreActivity        = "com.android.vending.AssetBrowserActivity"
		progressBarClassName     = "android.widget.ProgressBar"
		searchIconID             = "com.android.vending:id/search_icon"
		searchInputID            = "com.android.vending:id/search_bar_text_input"
		titleID                  = "com.example.android.architecture.blueprints.todomvp:id/title"
		userNameID               = "com.google.android.calendar:id/tile"
		defaultUITimeout         = 20 * time.Second
		longUITimeout            = 1000 * time.Minute
		mediumUITimeout          = 100 * time.Minute
	)

	a, err := arc.New(ctx, s.OutDir())
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()
	s.Log("Launch the playstore")
	if err := a.Command(ctx, "am", "start", "-W", playStorePkgName+"/"+playStoreActivity).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	must := func(err error) {
		if err != nil {
			path := filepath.Join(s.OutDir(), "launch-app-failed-test.png")
			if err := screenshot.Capture(ctx, path); err != nil {
				s.Log("screenshot for launch-app-failed-test: ", err)
			}
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}
	// Wait until the current activity is idle.
	must(d.WaitForIdle(ctx, 10*time.Second))

	//Check for search icon
	searchIcon := d.Object(ui.ID(searchIconID))
	searchIcon.WaitForExists(ctx, defaultUITimeout)
	must(searchIcon.Click(ctx))

	// click on search input
	searchInput := d.Object(ui.ID(searchInputID))
	must(searchInput.WaitForExists(ctx, defaultUITimeout))
	must(searchInput.Click(ctx))
	must(searchInput.SetText(ctx, appPkgName))

	must(d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0))

	// Click on apptitle
	appTitle := d.Object(ui.ID(playCardID))
	must(appTitle.WaitForExists(ctx, defaultUITimeout))
	must(appTitle.Click(ctx))

	// Click on Install button
	installButton := d.Object(ui.ClassName(installButtonClassName), ui.Text(installButtonText))
	installButton.WaitForExists(ctx, defaultUITimeout)
	must(installButton.Click(ctx))

	//Wait until progressbar is gone
	progressBar := d.Object(ui.ClassName(progressBarClassName), ui.PackageName(playStorePkgName))
	must(progressBar.WaitUntilGone(ctx, longUITimeout))

	// Click on open button
	openButton := d.Object(ui.ClassName(openButtonClassName), ui.Text(openButtonText))
	openButton.WaitForExists(ctx, mediumUITimeout)
	must(openButton.Click(ctx))

	s.Log("Launch the app")

	//Check for next icon
	nextIcon := d.Object(ui.ID(nextIconID))
	nextIcon.WaitForExists(ctx, defaultUITimeout)
	count := 0
	for count < 4 {
		if err := nextIcon.Exists(ctx); err != nil {
			s.Log("err2:", err)
		} else {
			nextIcon.Click(ctx)
		}
		count++
	}

	//Check for got it button
	gotItButton := d.Object(ui.ClassName(androidButtonClassName), ui.Text(gotItButtonText))
	gotItButton.WaitForExists(ctx, defaultUITimeout)
	if err := gotItButton.Exists(ctx); err != nil {
		s.Log("err3:", err)
	} else {
		gotItButton.Click(ctx)
	}

	//Click on allow button to access your calendar
	allowButton := d.Object(ui.ClassName(androidButtonClassName), ui.Text(allowButtonText))
	allowButton.WaitForExists(ctx, defaultUITimeout)
	if err := allowButton.Exists(ctx); err != nil {
		s.Log("err4:", err)
	} else {
		allowButton.Click(ctx)
	}
	//Click on allow button to access your contacts
	allowButton.WaitForExists(ctx, defaultUITimeout)
	if err := allowButton.Exists(ctx); err != nil {
		s.Log("err5:", err)
	} else {
		allowButton.Click(ctx)
	}
	// Click on hambugerIcon in home page of the app
	hamburgerIcon := d.Object(ui.ClassName(hamburgerIconClassName), ui.DescriptionContains(hamburgerIconDescription))
	hamburgerIcon.WaitForExists(ctx, defaultUITimeout)
	must(hamburgerIcon.Click(ctx))

	// check app is logged in with user
	userName := d.Object(ui.ID(userNameID))
	must(userName.WaitForExists(ctx, defaultUITimeout))

	// click on press back
	must(d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0))

	//check for add icon on home page
	addIcon := d.Object(ui.ClassName(addButtonClassName), ui.DescriptionContains(addButtonDescription))
	addIcon.WaitForExists(ctx, defaultUITimeout)
	must(addIcon.WaitForExists(ctx, 30*time.Second))

	//CleanUp, uninstall Google Calendar from the playstore
	defer func() {
		if err := a.Command(ctx, "pm", "uninstall", appPkgName).Run(); err != nil {
			s.Fatal("Failed to uninstall app: ", err)
		}
	}()
}
