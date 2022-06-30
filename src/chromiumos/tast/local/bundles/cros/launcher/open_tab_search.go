// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenTabSearch,
		Desc: "Test that Launcher search works with open tabs",
		Contacts: []string{
			"etuck@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
	})
}

func OpenTabSearch(ctx context.Context, s *testing.State) {
	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// A map from URL to the tab title it is expected to have.
	// E.g. the URL "chrome:version" is expected to have the tab title "About Version".
	expectedTabTitles := map[string]string{
		"about:blank":    "about:blank",
		"chrome:version": "About Version",
		"chrome:system":  "About System",
	}

	// Open a new window with the above tabs.
	for url := range expectedTabTitles {
		conn, err := cr.Browser().NewConn(ctx, url)
		if err != nil {
			s.Fatalf("Failed to open new window with url: %v, %v", url, err)
		}
		defer conn.Close()
	}

	// Check that the opened tabs have the expected titles.
	ui := uiauto.New(tconn)
	for _, expectedTabTitle := range expectedTabTitles {
		tabFinder := nodewith.Role(role.Tab).Name(expectedTabTitle)
		if err := ui.Exists(tabFinder)(ctx); err != nil {
			s.Fatalf("Failed to find tab with expected title: %v, %v", expectedTabTitle, err)
		}
	}

	for _, expectedTabTitle := range expectedTabTitles {
		// Search for an open tab in the productivity launcher and click on the result.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
		searchResultFinder := launcher.SearchResultListItemFinder.Ancestor(
			launcher.SearchResultListViewFinder.Name("Best Match , search result category"),
		).NameRegex(regexp.MustCompile(expectedTabTitle + ".*Go to this tab"))
		if err := uiauto.Combine("search for open tab and click on search result",
			launcher.Search(tconn, kb, expectedTabTitle),
			ui.WaitUntilExists(searchResultFinder),
			ui.LeftClick(searchResultFinder))(ctx); err != nil {
			s.Fatal("Failed to search for open tab and click on search result: ", err)
		}

		// Verify that searched-for tab is selected in the browser window.
		tabFinder := nodewith.Role(role.Tab).HasClass("Tab").Name(expectedTabTitle)
		tabInfo, err := ui.Info(ctx, tabFinder)
		if err != nil {
			s.Fatal("Failed to get info on tab: ", err)
		}
		if !tabInfo.Selected {
			s.Fatalf("Expected tab named %q to be selected but it wasn't", expectedTabTitle)
		}
	}
}
