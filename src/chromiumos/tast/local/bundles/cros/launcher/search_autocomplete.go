// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchAutocomplete,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks Autocomplete behavior in Launcher Search",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"yulunwu@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "clamshell_mode",
			Fixture: "chromeLoggedInExtendedAutocomplete",
			Val:     launcher.TestCase{TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Fixture:           "chromeLoggedInExtendedAutocomplete",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

type searchAutocompleteTestCase struct {
	searchKeyword          string
	result                 string
	category               string
	expectedSearchBoxText  string
	expectedGhostGhostText string
}

// SearchAutocomplete checks launcher search box behavior for autocompleting
// for highest ranked result.
func SearchAutocomplete(ctx context.Context, s *testing.State) {
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

	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, testCase.TabletMode, false /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	subtests := []searchAutocompleteTestCase{
		{
			searchKeyword:          "Web",
			result:                 "Web Store, Installed App",
			category:               "Best Match , search result category",
			expectedSearchBoxText:  "Web Store",
			expectedGhostGhostText: "TODO",
		},
		{
			searchKeyword:          "Store",
			result:                 "Web Store, Installed App",
			category:               "Best Match , search result category",
			expectedSearchBoxText:  "Store",
			expectedGhostGhostText: "TODO",
		},
		{
			searchKeyword:          "Youtub",
			result:                 "YouTube, Video sharing company - Google Search, Google Search",
			category:               "Best Match , search result category",
			expectedSearchBoxText:  "Youtube",
			expectedGhostGhostText: "TODO",
		},
		{
			searchKeyword:          "outube",
			result:                 "YouTube, Video sharing company - Google Search, Google Search",
			category:               "Best Match , search result category",
			expectedSearchBoxText:  "outube",
			expectedGhostGhostText: "TODO",
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.searchKeyword, func(ctx context.Context, s *testing.State) {
			ui := uiauto.New(tconn)
			clearSearchButton := nodewith.Role(role.Button).Name("Clear searchbox text")
			defer ui.LeftClick(clearSearchButton)(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_query_"+string(subtest.searchKeyword))

			if err := uiauto.Combine("search launcher",
				launcher.Search(tconn, kb, subtest.searchKeyword),
				launcher.WaitForCategorizedResult(tconn, subtest.category, subtest.result),
			)(ctx); err != nil {
				s.Fatal("Failed to search: ", err)
			}
			res, err :=
				launcher.CheckSearchBoxGhostText(ctx, tconn, subtest.expectedGhostGhostText)
			if err != nil {
				s.Fatal("Failed to find check ghost text: ", err)
			}

			if !res {
				s.Fatal("Failed to verify ghost text: ", subtest.expectedGhostGhostText)
			}
		})
	}
}
