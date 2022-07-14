// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Func:         CategoricalSearch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks for best match search results in the launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
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

type categoricalSearchTestCase struct {
	searchKeyword string
	category      string
	categoryLabel string
	result        string
}

// CategoricalSearch checks inline answers for special queries.
func CategoricalSearch(ctx context.Context, s *testing.State) {
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

	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, testCase.TabletMode, true /*productivityLauncher*/, false /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	subtests := []categoricalSearchTestCase{
		{
			searchKeyword: "Chrome",
			category:      "Best Match , search result category",
			categoryLabel: "Best Match",
			result:        "Chrome, Installed App",
		},
		{
			searchKeyword: "Settings",
			category:      "Best Match , search result category",
			categoryLabel: "Best Match",
			result:        "Settings, Installed App",
		},
		{
			searchKeyword: "Files",
			category:      "Best Match , search result category",
			categoryLabel: "Best Match",
			result:        "Files, Installed App",
		},
		{
			searchKeyword: "Shortcuts",
			category:      "Best Match , search result category",
			categoryLabel: "Best Match",
			result:        "Shortcuts, App",
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.searchKeyword, func(ctx context.Context, s *testing.State) {
			ui := uiauto.New(tconn)
			clearSearchButton := nodewith.Role(role.Button).Name("Clear searchbox text")
			defer ui.LeftClick(clearSearchButton)(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.searchKeyword))

			if err := uiauto.Combine("search launcher",
				launcher.Search(tconn, kb, subtest.searchKeyword),
				launcher.WaitForCategoryLabel(tconn, subtest.category, subtest.categoryLabel),
				launcher.WaitForCategorizedResult(tconn, subtest.category, subtest.result),
			)(ctx); err != nil {
				s.Fatal("Failed to search: ", err)
			}
		})
	}
}
