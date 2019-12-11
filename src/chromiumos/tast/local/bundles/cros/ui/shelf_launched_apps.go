// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShelfLaunchedApps,
		Desc: "Checks that launched apps appear in the shelf",
		Contacts: []string{
			"dhaddock@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// ShelfLaunchedApps tests that apps launched appear in the ChromeOS shelf.
func ShelfLaunchedApps(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// At login, we should have just Chrome in the Shelf.
	shelfItems, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	if len(shelfItems) != 1 {
		s.Fatal("Unexpected apps in the shelf. Expected only Chrome: ", shelfItems)
	}

	// Chrome must be first because it is automatically opened upon login.
	defaultApps := []apps.App{apps.Chrome, apps.Files, apps.WallpaperPicker}

	for _, app := range defaultApps {
		s.Logf("Launching %s", app.Name)
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	// Get the list of apps in the shelf via API.
	shelfItems, err = ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}

	s.Log("Checking that all expected apps are in the shelf")
	if len(shelfItems) != len(defaultApps) {
		s.Fatalf("Shelf items count does not match expected apps. Got: %v; Want: %v", len(shelfItems), len(defaultApps))
	}
	for i, shelfItem := range shelfItems {
		expectedApp := defaultApps[i]
		if shelfItem.AppID != expectedApp.ID {
			s.Errorf("App IDs did not match. Got: %v; Want: %v", shelfItem.AppID, expectedApp.ID)
		}
		if shelfItem.Title != expectedApp.Name {
			s.Errorf("App names did not match. Got: %v; Want: %v", shelfItem.Title, expectedApp.Name)
		}
	}

	// Get the list of apps in the shelf via UI.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get UI automation root: ", err)
	}
	defer root.Release(ctx)
	params := ui.FindParams{
		Role: "button",
	}
	name := "name"
	attributes := []string{name}
	icons, err := root.DescendantAttributes(ctx, params, attributes)
	if err != nil {
		s.Fatal("Failed to get all buttons: ", err)
	}

	// Check that the icons are also present in the UI
	for _, app := range defaultApps {
		var found = false
		for _, icon := range icons {
			if icon[name] == app.Name {
				s.Logf("Found icon for %s", app.Name)
				found = true
				break
			}
		}
		if !found {
			s.Errorf("There was no icon for %s in the shelf", app.Name)
		}
	}
}
