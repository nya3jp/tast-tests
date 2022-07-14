// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
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
		Desc:         "Tests that searching for queries associated with a keyhboard shortcut returns a keyboard shortcut result",
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
	// Search keyword.
	searchKeyword string
	// The result item name that is expected to be found within search results.
	result string
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
	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, testCase.TabletMode, testCase.ProductivityLauncher, false /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	subtests := []shortcutSearchTestCase{
		{
			searchKeyword: "Lock Screen",
			result:        "Lock screen, Shortcuts, Search+ l",
		},
		{
			searchKeyword: "Launcher",
			result:        "Open/close the launcher, Shortcuts, Search",
		},
		{
			searchKeyword: "Overview",
			result:        "Overview mode, Shortcuts, Overview mode key",
		},
		{
			searchKeyword: "new window",
			result:        "Open new window, Shortcuts, Ctrl+ n",
		},
		{
			searchKeyword: "new window",
			result:        "Open a new window in Incognito mode, Shortcuts, Ctrl+ Shift+ n",
		},
		{
			searchKeyword: "incognito",
			result:        "Open a new window in Incognito mode, Shortcuts, Ctrl+ Shift+ n",
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

			if err := kb.TypeKeyAction(input.KEY_ENTER)(ctx); err != nil {
				s.Fatal("Failed to launch first search result: ", err)
			}

			if err := ash.WaitForApp(ctx, tconn, apps.KeyboardSV.ID, time.Minute); err != nil {
				s.Fatal("Keyboard Shortcut Viewer failed to open: ", err)
			}

			if running, err := ash.AppRunning(ctx, tconn, apps.KeyboardSV.ID); err != nil {
				s.Fatal("Failed to check if Keyboard Shortcut View is running: ", err)
			} else if !running {
				s.Fatal("KeyboardSV not running: ", err)
			}

			if err := apps.Close(ctx, tconn, apps.KeyboardSV.ID); err != nil {
				s.Fatal("Failed to close Keyboard Shortcut View: ", err)
			}

			if err := ash.WaitForAppClosed(ctx, tconn, apps.KeyboardSV.ID); err != nil {
				s.Fatal("Keyboard Shortcut View did not close successfully: ", err)
			}
		})
	}
}
