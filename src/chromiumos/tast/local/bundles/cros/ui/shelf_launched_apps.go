// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/ui/apps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShelfLaunchedApps,
		Desc: "Checks that launched apps appear in the shelf",
		Contacts: []string{
			"dhaddock@chromium.org",
		},
		Attr:         []string{"informational"},
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

	// At login, we should have just Chrome in the Shelf
	shelfItems, err := ash.GetShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	if len(shelfItems) != 1 {
		s.Fatalf("Unexpected apps in the shelf. Expected only Chrome: %s", shelfItems)
	}

	defaultApps := [3]apps.App{apps.Chrome, apps.Files, apps.WallpaperPicker}

	s.Log("Launching some apps")
	for _, app := range defaultApps {
		if err := apps.LaunchApp(ctx, tconn, app.ID); err != nil {
			s.Errorf("Failed to launch: %s. %s", app.Name, err)
		}
		err = ash.WaitForAppToAppear(ctx, tconn, app.ID)
		if err != nil {
			s.Errorf("%s did not appear in shelf after launch. %s", app.Name, err)
		}
	}

	// Get the list of apps in the shelf via API and UI
	shelfItems, err = ash.GetShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	var icons []string
	findQuery := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.findAll({attributes: {role: 'button'}}).map(node => node.name))")
	if err := tconn.EvalPromise(ctx, findQuery, &icons); err != nil {
		s.Fatal("Failed to grab buttons on screen: ", err)
	}

	s.Log("Checking that all expected apps are in the shelf ")
	for i := range shelfItems {
		if defaultApps[i].ID != shelfItems[i].AppID {
			s.Errorf("App IDs did not match. Got: %v; Expected: %v", shelfItems[i].AppID, defaultApps[i].ID)
		}
		if defaultApps[i].Name != shelfItems[i].Title {
			s.Errorf("App names did not match. Got: %v; Expected: %v", shelfItems[i].Title, defaultApps[i].Name)
		}
		// Check that the icons are also present in the UI
		found := false
		for _, icon := range icons {
			if icon == defaultApps[i].Name {
				found = true
				break
			}
		}
		if !found {
			s.Errorf("There was no icon for %s in the shelf", defaultApps[i].Name)
		}
	}
}
