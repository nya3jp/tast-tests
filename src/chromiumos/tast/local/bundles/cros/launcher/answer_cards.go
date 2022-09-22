// Copyright 2022 The ChromiumOS Authors
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
			Name:    "definition_card_clamshell",
			Fixture: "chromeLoggedIn",
			Val: launcher.SearchTestCase{TabletMode: false,
				SearchKeyword:  "definition of flaky",
				ExpectedResult: launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile("/.*/")),
				Retries:        3,
			},
		}, {
			Name:    "translation_card_clamshell",
			Fixture: "chromeLoggedIn",
			Val: launcher.SearchTestCase{TabletMode: false,
				SearchKeyword:  "translate hello into spanish",
				ExpectedResult: launcher.SearchResultListItemFinder.NameContaining("Hola"),
				Retries:        3,
			},
		}, {
			Name:    "addition_card_clamshell",
			Fixture: "chromeLoggedIn",
			Val: launcher.SearchTestCase{TabletMode: false,
				SearchKeyword:  "1+1",
				ExpectedResult: launcher.SearchResultListItemFinder.NameContaining("1+1, 2"),
				Retries:        3,
			},
		}, {
			Name:    "unit_conversion_card_clamshell",
			Fixture: "chromeLoggedIn",
			Val: launcher.SearchTestCase{TabletMode: false,
				SearchKeyword:  "455 lb in kg",
				ExpectedResult: launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile("455 lb in kg, 206.*")),
				Retries:        3,
			},
		}, {
			Name:    "stock_card_clamshell",
			Fixture: "chromeLoggedIn",
			Val: launcher.SearchTestCase{TabletMode: false,
				SearchKeyword:  "goog stock",
				ExpectedResult: launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile("NASDAQ")),
				Retries:        3,
			},
		}, {
			Name:    "weather_card_clamshell",
			Fixture: "chromeLoggedIn",
			Val: launcher.SearchTestCase{TabletMode: false,
				SearchKeyword:  "weather",
				ExpectedResult: launcher.SearchResultListItemFinder.NameRegex(regexp.MustCompile("-?[1-9][0-9]*")),
				Retries:        3,
			},
		},
		/* Disabled due to <1% pass rate over 30 days. See b/241943050
		{
			Name:              "tablet_mode",
			Fixture:           "chromeLoggedIn",
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

	testCase := s.Param().(launcher.SearchTestCase)
	tabletMode := testCase.TabletMode

	// SetUpLauncherTest opens the launcher.
	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, tabletMode, false /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	ui := uiauto.New(tconn)

	for i := 0; i < testCase.Retries; i++ {
		failed := false
		s.Run(ctx, testCase.SearchKeyword, func(ctx context.Context, s *testing.State) {
			if err := uiauto.Combine("search launcher",
				launcher.Search(tconn, kb, testCase.SearchKeyword),
				ui.WaitUntilExists(testCase.ExpectedResult),
			)(ctx); err != nil {
				failed = true
				s.Log("Failed to search: ", err)
				faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(testCase.SearchKeyword))
			}
		})
		if !failed {
			return
		}
		// Clear the input and try again.
		kb.TypeKey(ctx, input.KEY_ESC)
	}
	s.Fatal("Failed to search: ", testCase.SearchKeyword)
}
