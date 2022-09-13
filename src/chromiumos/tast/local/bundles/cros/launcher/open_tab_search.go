// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type openTabSearchTestCase struct {
	tabletMode     bool            // whether the test runs in tablet mode
	tabTitle       string          // title of the tab to search for
	browserConfigs []browserConfig // the browser setup for this test case
}

type browserConfig struct {
	tabURLs   []string // list of tab URLs to open in this browser
	minimized bool     // whether this browser should be minimized
}

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenTabSearch,
		Desc: "Test that Launcher search works with open tabs",
		Contacts: []string{
			"etuck@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "all_tabs_in_one_window",
			Fixture: "chromeLoggedIn",
			Val: &openTabSearchTestCase{
				tabletMode: false,
				tabTitle:   "Google",
				browserConfigs: []browserConfig{{
					tabURLs:   []string{"about:blank", "chrome:version", "https://www.google.com", "chrome:credits"},
					minimized: false,
				},
				},
			},
		}, {
			Name:              "all_tabs_in_one_window_tablet_mode",
			Fixture:           "chromeLoggedIn",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			Val: &openTabSearchTestCase{
				tabletMode: true,
				tabTitle:   "Google",
				browserConfigs: []browserConfig{{
					tabURLs:   []string{"about:blank", "chrome:version", "https://www.google.com", "chrome:credits"},
					minimized: false,
				},
				},
			},
		}, {
			Name:    "all_tabs_in_one_window_minimized",
			Fixture: "chromeLoggedIn",
			Val: &openTabSearchTestCase{
				tabletMode: false,
				tabTitle:   "Google",
				browserConfigs: []browserConfig{{
					tabURLs:   []string{"about:blank", "chrome:version", "https://www.google.com", "chrome:credits"},
					minimized: true,
				},
				},
			},
		}, {
			Name:              "all_tabs_in_one_window_minimized_tablet_mode",
			Fixture:           "chromeLoggedIn",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			Val: &openTabSearchTestCase{
				tabletMode: true,
				tabTitle:   "Google",
				browserConfigs: []browserConfig{{
					tabURLs:   []string{"about:blank", "chrome:version", "https://www.google.com", "chrome:credits"},
					minimized: true,
				},
				},
			},
		}, {
			Name:    "multiple_windows",
			Fixture: "chromeLoggedIn",
			Val: &openTabSearchTestCase{
				tabletMode: false,
				tabTitle:   "About Version",
				browserConfigs: []browserConfig{{
					tabURLs:   []string{"about:blank", "chrome:credits"},
					minimized: false,
				}, {
					tabURLs:   []string{"chrome:version", "https://www.google.com"},
					minimized: false,
				},
				},
			},
		}, {
			Name:              "multiple_windows_tablet_mode",
			Fixture:           "chromeLoggedIn",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			Val: &openTabSearchTestCase{
				tabletMode: true,
				tabTitle:   "About Version",
				browserConfigs: []browserConfig{{
					tabURLs:   []string{"about:blank", "chrome:credits"},
					minimized: false,
				}, {
					tabURLs:   []string{"chrome:version", "https://www.google.com"},
					minimized: false,
				},
				},
			},
		}, {
			Name:    "multiple_windows_minimized",
			Fixture: "chromeLoggedIn",
			Val: &openTabSearchTestCase{
				tabletMode: false,
				tabTitle:   "About Version",
				browserConfigs: []browserConfig{{
					tabURLs:   []string{"about:blank", "chrome:credits"},
					minimized: false,
				}, {
					tabURLs:   []string{"chrome:version", "https://www.google.com"},
					minimized: true,
				},
				},
			},
		}, {
			Name:              "multiple_windows_minimized_tablet_mode",
			Fixture:           "chromeLoggedIn",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			Val: &openTabSearchTestCase{
				tabletMode: true,
				tabTitle:   "About Version",
				browserConfigs: []browserConfig{{
					tabURLs:   []string{"about:blank", "chrome:credits"},
					minimized: false,
				}, {
					tabURLs:   []string{"chrome:version", "https://www.google.com"},
					minimized: true,
				},
				},
			},
		},
		},
	})
}

func OpenTabSearch(ctx context.Context, s *testing.State) {
	tcName := s.TestName()
	testCase := s.Param().(*openTabSearchTestCase)
	tabletMode := testCase.tabletMode
	expectedTabTitle := testCase.tabTitle
	browserConfigs := testCase.browserConfigs

	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump_subtest_"+tcName)

	ui := uiauto.New(tconn)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// Prepare the browser(s).
	for _, browserConfig := range browserConfigs {
		conn, err := setupBrowser(ctx, s, tconn, ui, cr, browserConfig)
		if err != nil {
			s.Fatal("Failed to set up browser: ", err)
		}
		defer conn.Close()
	}

	// Search for an open tab in the productivity launcher and click on the result.
	if err := launcher.OpenProductivityLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open bubble launcher: ", err)
	}
	searchResultFinder := launcher.SearchResultListItemFinder.NameRegex(
		regexp.MustCompile(expectedTabTitle + ".*Go to this tab"))
	if err := uiauto.Combine("search for open tab and click on search result",
		launcher.Search(tconn, kb, expectedTabTitle),
		ui.WaitUntilExists(searchResultFinder),
		ui.LeftClick(searchResultFinder))(ctx); err != nil {
		s.Fatal("Failed to search for open tab and click on search result: ", err)
	}

	// Verify that searched-for tab and the browser window within which it appears are both active.
	_, err = ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.WindowType == ash.WindowTypeBrowser && w.IsActive && regexp.MustCompile(expectedTabTitle).MatchString(w.Title)
	})
	if err != nil {
		s.Fatalf("Failed to find active window with title having %q as a substring: %v", expectedTabTitle, err)
	}
	if !tabletMode {
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

// setupBrowser sets up a single browser according to the provided config. This
// returns a connectiong to the newly-created Chrome renderer, or nil if there
// is an error.
// This will fail if attempting to populate a browser with no tabs, or if
// something goes wrong while loading a tab.
func setupBrowser(ctx context.Context, s *testing.State, tconn *chrome.TestConn, ui *uiauto.Context, cr *chrome.Chrome, config browserConfig) (*browser.Conn, error) {
	urls := config.tabURLs
	if len(urls) < 1 {
		return nil, errors.New("Attempted to populate browser with fewer than one tab")
	}
	b := cr.Browser()
	conn, err := b.NewConn(ctx, urls[0], browser.WithNewWindow())
	window, err := ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.WindowType == ash.WindowTypeBrowser && w.IsActive
	})
	for _, url := range urls[1:] {
		if conn, err = b.NewConn(ctx, url); err != nil {
			return nil, err
		}
	}
	if config.minimized {
		if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateMinimized); err != nil {
			return nil, err
		}
	}
	return conn, nil
}
