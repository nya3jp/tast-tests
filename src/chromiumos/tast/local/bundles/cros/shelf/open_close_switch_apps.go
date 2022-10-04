// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenCloseSwitchApps,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks interacting with apps in the shelf",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

// Basic info about the apps used in this test, used in several verification steps.
// Grouping these together helps simplify the flow of the test.
type appInfo struct {
	ShelfBtn    *nodewith.Finder
	ID          string
	WindowTitle string
	Name        string
}

// OpenCloseSwitchApps verifies that we can launch, switch between, and close apps from the shelf.
func OpenCloseSwitchApps(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	bt := s.Param().(browser.Type)

	// Test acts different in clamshell or tablet mode.
	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode status: ", err)
	}
	var tc *pointer.TouchContext
	var tsew *input.TouchscreenEventWriter
	var tcc *input.TouchCoordConverter
	var stw *input.SingleTouchEventWriter
	if tabletMode {
		tc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create the touch controller: ", err)
		}
		defer tc.Close()
		tsew, tcc, err = touch.NewTouchscreenAndConverter(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to access to the touchscreen: ", err)
		}
		defer tsew.Close()
		stw, err = tsew.NewSingleTouchWriter()
		if err != nil {
			s.Fatal("Failed to create the single touch writer: ", err)
		}
	}

	// The test account has only Chrome pinned to the shelf, so we'll have to
	// launch and pin another app.
	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch the Files app: ", err)
	}
	if err := ash.PinApp(ctx, tconn, apps.FilesSWA.ID); err != nil {
		s.Fatal("Failed to pin Files app to the shelf: ", err)
	}

	// Get the expected browser app info.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatalf("Could not find the %v browser app: %v", bt, err)
	}
	// Chrome app name doesn't exactly match the chrome shelf name so modify it here for simpler code later.
	if browserApp.ID == apps.Chrome.ID {
		browserApp.Name = "Google Chrome"
	}

	// Find the shelf icon buttons.
	ui := uiauto.New(tconn)
	chromeBtn := nodewith.ClassName("ash/ShelfAppButton").Name(browserApp.Name)
	filesBtn := nodewith.ClassName("ash/ShelfAppButton").Name(apps.Files.Name)
	if err := ui.WaitUntilExists(chromeBtn)(ctx); err != nil {
		s.Fatal("Failed to find Chrome shelf button: ", err)
	}
	if err := ui.WaitUntilExists(filesBtn)(ctx); err != nil {
		s.Fatal("Failed to find Files shelf button: ", err)
	}

	chromeInfo := appInfo{chromeBtn, browserApp.ID, "New Tab", browserApp.Name}
	filesInfo := appInfo{filesBtn, apps.FilesSWA.ID, "Files - My files", apps.Files.Name}
	checkApps := []appInfo{chromeInfo, filesInfo}

	// Close the apps so we can try opening them from the shelf.
	// Chrome is launched by default, so it needs to be closed, too.
	for _, app := range checkApps {
		if err := apps.Close(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to close %s: %s", app.Name, err)
		}
		if err := ash.WaitForAppClosed(ctx, tconn, app.ID); err != nil {
			s.Fatalf("%s did not close successfully: %s", app.Name, err)
		}
	}
	// Click the apps in the shelf and see if they open.
	// Repeat a second time to make sure we can switch focus between them once opened.
	for i := 0; i < 2; i++ {
		for _, app := range checkApps {
			if tabletMode {
				if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
					s.Fatal("Failed to swipe up the hotseat: ", err)
				}
			}
			if err := ui.LeftClick(app.ShelfBtn)(ctx); err != nil {
				s.Fatalf("Failed to click %v shelf button: %v", app.Name, err)
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.IsActive && strings.Contains(w.Title, app.WindowTitle)
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				if i == 0 {
					s.Fatalf("%v app window not opened after clicking shelf icon: %v", app.Name, err)
				} else {
					s.Fatalf("%v app window not focused after clicking shelf icon: %v", app.Name, err)
				}
			}
		}
	}

	// Close the apps via shelf context menu
	for _, app := range checkApps {
		if tabletMode {
			if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
				s.Fatal("Failed to swipe up the hotseat: ", err)
			}
		}
		if err := ui.RightClick(app.ShelfBtn)(ctx); err != nil {
			s.Fatalf("Failed to right-click %v shelf button: %v", app.Name, err)
		}

		closeBtn := nodewith.Role(role.MenuItem).Name("Close")
		if err := ui.WaitUntilExists(closeBtn)(ctx); err != nil {
			s.Fatalf("Failed to find Close option in %v shelf icon context menu: %v", app.Name, err)
		}

		// The 'Close' button is not immediately clickable after we context-click,
		// so keep clicking until it goes away, indicating it has been clicked.
		if err := ui.LeftClickUntil(closeBtn, ui.Gone(closeBtn))(ctx); err != nil {
			s.Fatalf("Failed to click Close in %v shelf icon context menu: %v", app.Name, err)
		}
		if err := ash.WaitForAppClosed(ctx, tconn, app.ID); err != nil {
			s.Errorf("%v still open after trying to close it from the shelf context menu: %v", app.Name, err)
		}
	}
}
