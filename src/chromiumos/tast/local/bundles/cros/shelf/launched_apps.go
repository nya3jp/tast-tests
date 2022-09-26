// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchedApps,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that launched apps appear in the shelf",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},

		Params: []testing.Param{
			{
				// Primary form factor is not tablet.
				Name:              "",
				Fixture:           "chromeLoggedInDisableSync",
				Val:               false,
				ExtraSoftwareDeps: []string{"no_tablet_form_factor"},
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "lacros",
				Fixture:           "lacrosDisableSync",
				Val:               false,
				ExtraSoftwareDeps: []string{"lacros", "no_tablet_form_factor"},
				ExtraAttr:         []string{"informational"},
			},
			{
				// Primary form factor is tablet.
				Name:              "tablet_form_factor",
				Fixture:           "chromeLoggedInDisableSync",
				Val:               true,
				ExtraSoftwareDeps: []string{"tablet_form_factor"},
			},
			{
				Name:              "tablet_form_factor_lacros",
				Fixture:           "lacrosDisableSync",
				Val:               true,
				ExtraSoftwareDeps: []string{"lacros", "tablet_form_factor"},
			},
		},
	})
}

// LaunchedApps tests that apps launched appear in the ChromeOS shelf.
func LaunchedApps(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Get the expected browser app info.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the browser app: ", err)
	}

	// Chrome app name doesn't exactly match the chrome shelf name so modify it here for simpler code later.
	if browserApp.ID == apps.Chrome.ID {
		browserApp.Name = "Google Chrome"
	}

	tabletMode := s.Param().(bool)
	var defaultAppsPartial []apps.App
	if tabletMode {
		defaultAppsPartial = []apps.App{browserApp}
	} else {
		defaultAppsPartial = []apps.App{browserApp, apps.FilesSWA}
	}
	defaultAppsFull := append(defaultAppsPartial, apps.Gmail, apps.Docs, apps.Youtube)

	// Check that default apps are already pinned once logged in.
	shelfItems, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}

	var defaultApps []apps.App
	if len(shelfItems) == len(defaultAppsPartial) {
		defaultApps = defaultAppsPartial
	} else if len(shelfItems) == len(defaultAppsFull) {
		defaultApps = defaultAppsFull
	} else {
		s.Fatalf("Unexpected number of apps in shelf, expected  %d or %d, got: %d", len(defaultAppsPartial), len(defaultAppsFull), len(shelfItems))
	}

	for _, app := range defaultApps {
		s.Logf("Launching %s", app.Name)
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
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

	ui := uiauto.New(tconn)

	// Check that the icons are also present in the UI
	for _, app := range defaultApps {
		err := ui.Exists(nodewith.ClassName(ash.ShelfIconClassName).Role(role.Button).Name(app.Name))(ctx)
		if err != nil {
			s.Errorf("There was no icon for %s in the shelf", app.Name)
		} else {
			s.Logf("Found icon for %s", app.Name)
		}
	}
}
