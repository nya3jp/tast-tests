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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AnswerCards,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks for omnibox answer cards in the launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"yulunwu@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "productivity_launcher_clamshell_mode",
			Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
			Val:     launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Fixture:           "chromeLoggedInWith100FakeAppsProductivityLauncher",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

type answerCardTestCase struct {
	searchKeyword  string
	validateAction uiauto.Action
}

// AnswerCards checks inline answers for special queries.
func AnswerCards(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

	var subtests = []answerCardTestCase{
		{
			searchKeyword:  "definition of agriculture",
			validateAction: uiauto.New(tconn).WaitUntilExists(launcher.SearchResultListItemFinder.NameContaining("/ˈaɡrəˌkəlCHər/")),
		},
		{
			searchKeyword:  "1+1",
			validateAction: uiauto.New(tconn).WaitUntilExists(launcher.SearchResultListItemFinder.NameContaining("1+1, 2")),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, string(subtest.searchKeyword), func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.searchKeyword))

			if err := uiauto.Combine("search launcher",
				launcher.Open(tconn),
				launcher.Search(tconn, kb, subtest.searchKeyword),
				subtest.validateAction,
			)(ctx); err != nil {
				s.Fatal("Failed to search: ", err)
			}

			// Close launcher search by sending KEY_ESC.
			if err := kb.TypeKey(ctx, input.KEY_ESC); err != nil {
				s.Fatalf("Failed to send %d: %v", input.KEY_UP, err)
			}
		})
	}
}
