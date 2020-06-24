// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShelfOpenCloseSwitchApps,
		Desc: "Checks basic shelf functionality",
		Contacts: []string{
			"kyleshima@chromium.org",
			"bhansknecht@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
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

// ShelfOpenCloseSwitchApps verifies that we can launch, switch between, and close apps from the shelf.
func ShelfOpenCloseSwitchApps(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// The test account has only Chrome pinned to the shelf, so we'll have to
	// launch and pin another app.
	if err := apps.Launch(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Files app did not appear in shelf after launch: ", err)
	}
	if err := ash.PinApp(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Failed to pin Files app to the shelf: ", err)
	}

	// Find the shelf icon buttons.
	// A short sleep here ensures the app icons are in the right place before we locate them.
	// Without it, the test moves too quickly and will try to click the center of the shelf
	// later, where the Chrome icon is initially located.
	testing.Sleep(ctx, time.Second)
	chromeBtn, err := ui.Find(ctx, tconn, ui.FindParams{ClassName: "ash/ShelfAppButton", Name: apps.Chrome.Name})
	if err != nil {
		s.Fatal("Failed to find Chrome shelf button: ", err)
	}
	defer chromeBtn.Release(ctx)

	filesBtn, err := ui.Find(ctx, tconn, ui.FindParams{ClassName: "ash/ShelfAppButton", Name: apps.Files.Name})
	if err != nil {
		s.Fatal("Failed to find Files shelf button: ", err)
	}
	defer filesBtn.Release(ctx)

	chromeInfo := appInfo{chromeBtn, apps.Chrome.ID, "Chrome - New Tab", "apps.Chrome.Name"}
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
			if err := app.ShelfBtn.LeftClick(ctx); err != nil {
				s.Fatalf("Failed to click %v shelf button: %v", app.Name, err)
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.IsActive && w.Title == app.WindowTitle
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
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
