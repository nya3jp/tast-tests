// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// removalDialogFinder is a node finder for launcher dialog shown to confirm suggested search
// result from launcehr search.
var removalDialogFinder = nodewith.Role(role.AlertDialog).NameContaining("Remove this suggestion")

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveSuggestedSearchResult,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the user us able to remove omnibox search suggestions from launcher search UI",
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
			Fixture: "chromeLoggedIn",
			Val:     launcher.TestCase{TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Fixture:           "chromeLoggedIn",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// RemoveSuggestedSearchResult verifies that omnibox results for suggested searches can be removed
// from launcher search. The test first opens a google search for a test specific search query,
// verifies the query starts appearing in launcher search, and removes it from search using launcher
// UI.
func RemoveSuggestedSearchResult(ctx context.Context, s *testing.State) {
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

	testQuery := fmt.Sprintf("testquery_to_remove_%t", tabletMode)

	// Open chrome window, and enter search query into omnibox.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	conn, err := br.NewConn(ctx, fmt.Sprintf("https://www.google.com/search?q=%s", testQuery))
	if err != nil {
		s.Fatal("Failed to open new connection: ", err)
	}
	defer conn.Close()

	browserRootFinder := nodewith.Role(role.Window).HasClass("BrowserRootView")
	expectedNode := browserRootFinder.NameContaining(testQuery)

	if err := uiauto.New(tconn).WaitUntilExists(expectedNode)(ctx); err != nil {
		s.Fatal("Failed to verify test query was handled: ", err)
	}

	// SetUpLauncherTest opens the launcher.
	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, tabletMode, false /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	resultFinder := launcher.SearchResultListItemFinder.Name(fmt.Sprintf("%s, Google Search", testQuery))

	// Open the launcher, and search for the test query - verifies that the search yields the
	// associated "Google Search" result.
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("search launcher",
		launcher.Search(tconn, kb, testQuery),
		ui.WaitUntilExists(resultFinder),
	)(ctx); err != nil {
		s.Fatal("Failed to search: ", err)
	}

	removeButton := nodewith.Role(role.Button).Name(fmt.Sprintf("Remove the suggestion %s", testQuery))
	if err := uiauto.Combine("Trigger and cancel result removal",
		triggerSuggestionRemovalAction(tconn, resultFinder, removeButton, tabletMode),
		performSuggestionRemovalDialogAction(tconn, "Cancel"),
	)(ctx); err != nil {
		s.Fatal("Failed to cancel the removal dialog: ", err)
	}

	// Verify the search result does not get removed from the search UI.
	if err := ui.Exists(resultFinder)(ctx); err != nil {
		s.Fatal("Result removed from the UI: ", err)
	}
	if err := ui.WaitUntilGone(resultFinder)(ctx); err == nil {
		s.Fatal("Result removed from UI after a timeout")
	}

	// Search again, and verify the test query is still returned as a result
	if err := clearSearch(tconn)(ctx); err != nil {
		s.Fatal("Failed to clear search results: ", err)
	}

	if err := uiauto.Combine("search, and remove suggestion",
		launcher.Search(tconn, kb, testQuery),
		ui.WaitUntilExists(resultFinder),
		triggerSuggestionRemovalAction(tconn, resultFinder, removeButton, tabletMode),
		performSuggestionRemovalDialogAction(tconn, "Remove"),
		ui.WaitUntilGone(resultFinder),
		ui.EnsureGoneFor(resultFinder, 3*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to remove the test query from launcher search: ", err)
	}

	// Search again, and verify the test query is no longer returned as a result
	if err := uiauto.Combine("search after result removal",
		clearSearch(tconn),
		launcher.Search(tconn, kb, testQuery),
		ui.EnsureGoneFor(resultFinder, 3*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to verify removed result does not reappear in search: ", err)
	}

	// Close and reopen the launcher, verify search does not start returning the removed result.
	if tabletMode {
		if err := launcher.HideTabletModeLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to hide the launcher in tablet mode: ", err)
		}
	} else {
		if err := launcher.CloseBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to hide the launcher in clamshell mode: ", err)
		}
	}

	if err := uiauto.Combine("reopen launcher and search",
		launcher.Open(tconn),
		launcher.Search(tconn, kb, testQuery),
		ui.EnsureGoneFor(resultFinder, 3*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to verify test result does not appear after reopening launcher: ", err)
	}
}

// triggerSuggestionRemovalAction triggers result removal action for a search result.
// resultFinder is the node finder to find the target result view
// removeButton is the node finder for remove action button
// tabletMode indicates whether the test is running for tablet mode launcher
func triggerSuggestionRemovalAction(tconn *chrome.TestConn, resultFinder, removeButton *nodewith.Finder, tabletMode bool) uiauto.Action {
	ui := uiauto.New(tconn)

	// To get the removal action button to show up:
	// in clamshel, hover the mouse over the result view;
	// in tablet mode, long press the result view
	if tabletMode {
		return uiauto.Combine("activate remove button using touch",
			func(ctx context.Context) error {
				touchCtx, err := touch.New(ctx, tconn)
				if err != nil {
					return errors.Wrap(err, "Fail to get touch screen")
				}
				defer touchCtx.Close()

				return touchCtx.LongPress(resultFinder)(ctx)
			},
			ui.WaitUntilExists(removeButton),
			ui.LeftClick(removeButton),
			ui.WaitUntilExists(removalDialogFinder))
	}

	return uiauto.Combine("activate remove button using mouse",
		ui.MouseMoveTo(resultFinder, 10*time.Millisecond),
		ui.WaitUntilExists(removeButton),
		ui.LeftClick(removeButton),
		ui.WaitUntilExists(removalDialogFinder))
}

// performSuggestionRemovalDialogAction presses a dialog button with name dialogButtonName in the
// dialog shown to users when they attempt to remove suggested omnibox search result from launcher search.
// This will fail if the removal dialog is not shown.
func performSuggestionRemovalDialogAction(tconn *chrome.TestConn, dialogButtonName string) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine("press removal dialog button",
		ui.LeftClick(nodewith.Role(role.Button).Name(dialogButtonName).Ancestor(removalDialogFinder)),
		ui.WaitUntilGone(removalDialogFinder))
}

// clearSearch clicks the clear search button in launcher search UI.
func clearSearch(tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine("clear search",
		ui.LeftClick(nodewith.Role(role.Button).Name("Clear searchbox text")),
		ui.WaitUntilGone(launcher.SearchResultListItemFinder.First()))
}
