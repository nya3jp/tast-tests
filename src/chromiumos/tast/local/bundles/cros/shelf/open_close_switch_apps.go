// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenCloseSwitchApps,
		Desc: "Checks interacting with apps in the shelf",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// Basic info about the apps used in this test, used in several verification steps.
// Grouping these together helps simplify the flow of the test.
type appInfo struct {
	ShelfBtn    *ui.Node
	ID          string
	WindowTitle string
	Name        string
}

// OpenCloseSwitchApps verifies that we can launch, switch between, and close apps from the shelf.
func OpenCloseSwitchApps(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Test acts different in clamshell or tablet mode.
	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode status: ", err)
	}
	var tc *pointer.TouchController
	if tabletMode {
		tc, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create the touch controller: ", err)
		}
		defer tc.Close()
	}

	// The test account has only Chrome pinned to the shelf, so we'll have to
	// launch and pin another app.
	if err := apps.Launch(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Files.ID, time.Minute); err != nil {
		s.Fatal("Files app did not appear in shelf after launch: ", err)
	}
	if err := ash.PinApp(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Failed to pin Files app to the shelf: ", err)
	}

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	// Chrome app name doesn't exactly match the chrome shelf name so modify it here for simpler code later.
	if chromeApp.Name == apps.Chrome.Name {
		chromeApp.Name = "Google Chrome"
	}

	// Find the shelf icon buttons. StableFind ensures the shelf icons have stopped redistributing after launching the apps.
	opts := testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 10 * time.Second}
	chromeBtn, err := ui.StableFind(ctx, tconn, ui.FindParams{ClassName: "ash/ShelfAppButton", Name: chromeApp.Name}, &opts)
	if err != nil {
		s.Fatal("Failed to find Chrome shelf button: ", err)
	}
	defer chromeBtn.Release(ctx)

	filesBtn, err := ui.StableFind(ctx, tconn, ui.FindParams{ClassName: "ash/ShelfAppButton", Name: apps.Files.Name}, &opts)
	if err != nil {
		s.Fatal("Failed to find Files shelf button: ", err)
	}
	defer filesBtn.Release(ctx)

	chromeInfo := appInfo{chromeBtn, chromeApp.ID, "New Tab", chromeApp.Name}
	filesInfo := appInfo{filesBtn, apps.Files.ID, "Files - My files", apps.Files.Name}
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
				if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, tc.EventWriter(), tc.TouchCoordConverter()); err != nil {
					s.Fatal("Failed to swipe up the hotseat: ", err)
				}
			}
			if err := app.ShelfBtn.LeftClick(ctx); err != nil {
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
			if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, tc.EventWriter(), tc.TouchCoordConverter()); err != nil {
				s.Fatal("Failed to swipe up the hotseat: ", err)
			}
		}
		if err := app.ShelfBtn.RightClick(ctx); err != nil {
			s.Fatalf("Failed to right-click %v shelf button: %v", app.Name, err)
		}

		params := ui.FindParams{Role: ui.RoleTypeMenuItem, Name: "Close"}
		closeBtn, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
		if err != nil {
			s.Fatalf("Failed to find Close option in %v shelf icon context menu: %v", app.Name, err)
		}
		defer closeBtn.Release(ctx)

		// The 'Close' button is not immediately clickable after we context-click,
		// so keep clicking until it goes away, indicating it has been clicked.
		condition := func(ctx context.Context) (bool, error) {
			exists, err := ui.Exists(ctx, tconn, params)
			return !exists, err
		}
		opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}

		if err := closeBtn.LeftClickUntil(ctx, condition, &opts); err != nil {
			s.Fatalf("Failed to click Close in %v shelf icon context menu: %v", app.Name, err)
		}
		if err := ash.WaitForAppClosed(ctx, tconn, app.ID); err != nil {
			s.Errorf("%v still open after trying to close it from the shelf context menu: %v", app.Name, err)
		}
	}
}
