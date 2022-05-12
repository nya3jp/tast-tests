// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShortcutSearch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks for best match search results in the launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"yulunwu@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "clamshell_mode",
			Fixture: "chromeLoggedInWithProductivityLauncher",
			Val:     launcher.TestCase{TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Fixture:           "chromeLoggedInWithProductivityLauncher",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

type shortcutSearchTestCase struct {
	searchKeyword string
	result        string
	isFirstResult bool
}

// ShortcutSearch tests launcher searches for keyboard shortcuts.
func ShortcutSearch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed: ", err)
		}
	}

	subtests := []shortcutSearchTestCase{
		{
			searchKeyword: "Lock Screen",
			result:        "Lock screen, Shortcuts, Search+ l",
			isFirstResult: true,
		},
		{
			searchKeyword: "Launcher",
			result:        "Open/close the launcher, Shortcuts, Search",
			// Currently, Alt + Shift + l (to focus launcher button) is returned as the first result.
			isFirstResult: false,
		},
		{
			searchKeyword: "Overview",
			result:        "Overview mode, Shortcuts, Overview mode key",
			isFirstResult: true,
		},
		{
			searchKeyword: "new window",
			result:        "Open new window, Shortcuts, Ctrl+ n",
			isFirstResult: true,
		},
		{
			searchKeyword: "new window",
			result:        "Open a new window in Incognito mode, Shortcuts, Ctrl+ Shift+ n",
			isFirstResult: false,
		},
		{
			searchKeyword: "new incognito",
			result:        "Open a new window in Incognito mode, Shortcuts, Ctrl+ Shift+ n",
			isFirstResult: true,
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.searchKeyword, func(ctx context.Context, s *testing.State) {
			ui := uiauto.New(tconn)
			clearSearchButton := nodewith.Role(role.Button).Name("Clear searchbox text")

			defer ui.LeftClick(clearSearchButton)(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.searchKeyword))

			resultFinder := launcher.SearchResultListItemFinder.Name(subtest.result)
			if err := uiauto.Combine("search launcher",
				launcher.Open(tconn),
				launcher.Search(tconn, kb, subtest.searchKeyword),
				ui.WaitUntilExists(resultFinder),
			)(ctx); err != nil {
				s.Fatal("Failed to search: ", err)
			}

			if subtest.isFirstResult {
				firstResult := launcher.SearchResultListItemFinder.First()
				firstResultInfo, err := ui.Info(ctx, firstResult)
				if err != nil {
					s.Fatal("Failed to get first result info: ", err)
				}

				if firstResultInfo.Name != subtest.result {
					s.Fatalf("Expected search result %q not the best result - %q", subtest.result, firstResultInfo.Name)
				}
			}

			if err := uiauto.Combine("clear search",
				ui.LeftClick(clearSearchButton),
				ui.WaitUntilGone(resultFinder),
			)(ctx); err != nil {
				s.Fatal("Failed to clear search results: ", err)
			}
		})
	}
}
