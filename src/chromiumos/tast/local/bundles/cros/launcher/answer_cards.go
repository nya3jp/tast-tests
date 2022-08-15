// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
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
			Fixture: "chromeLoggedInWithProductivityLauncher",
			Val:     launcher.TestCase{TabletMode: false},
		},
		/* Disabled due to <1% pass rate over 30 days. See b/241943050
		{
			Name:              "productivity_launcher_tablet_mode",
			Fixture:           "chromeLoggedInWithProductivityLauncher",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}
		*/
		},
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

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode

	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, tabletMode, true /*productivityLauncher*/, false /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	ui := uiauto.New(tconn)

	subtests := []answerCardTestCase{
		{
			searchKeyword:  "definition of flaky",
			validateAction: ui.WaitUntilExists(launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile("/.*/"))),
		},
		{
			searchKeyword:  "hello in spanish",
			validateAction: ui.WaitUntilExists(launcher.SearchResultListItemFinder.NameContaining("Hola")),
		},
		{
			searchKeyword:  "1+1",
			validateAction: ui.WaitUntilExists(launcher.SearchResultListItemFinder.NameContaining("1+1, 2")),
		},
		{
			searchKeyword:  "is europe a continent",
			validateAction: ui.WaitUntilExists(launcher.SearchResultListItemFinder.NameContaining("is europe a continent, yes")),
		},
		{
			searchKeyword:  "455 lb in kg",
			validateAction: ui.WaitUntilExists(launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile("455 lb in kg, 206.*"))),
		},
		{
			searchKeyword:  "goog stock",
			validateAction: ui.WaitUntilExists(launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile("NASDAQ"))),
		},
		{
			searchKeyword:  "weather",
			validateAction: ui.WaitUntilExists(launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile("-?[1-9][0-9]*"))),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.searchKeyword, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.searchKeyword))

			if err := uiauto.Combine("search launcher",
				launcher.Open(tconn),
				launcher.Search(tconn, kb, subtest.searchKeyword),
				subtest.validateAction,
				launcher.CloseBubbleLauncher(tconn),
			)(ctx); err != nil {
				s.Fatal("Failed to search: ", err)
			}
		})
	}
}
