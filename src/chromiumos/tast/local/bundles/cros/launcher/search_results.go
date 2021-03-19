// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var stableModels = []string{
	// Top VK usage board in 2020 -- convertible, ARM.
	"hana",
	// Kukui family, not much usage, but very small tablet.
	"kodama",
	"krane",
	// Convertible chromebook, top usage in 2018 and 2019.
	"eve",
	"betty",
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SearchResults,
		Desc: "Launcher search contains Help content and omnibox result",
		Contacts: []string{
			"shengjun@google.com",
			"showoff-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: hwdep.D(hwdep.Model(stableModels...)),
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(stableModels...)),
		}},
	})
}

type testData struct {
	searchKeyword  string
	validateAction uiauto.Action
}

// SearchResults verifies launcher search returns Help content and omnibox result.
func SearchResults(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("EnableOmniboxRichEntities"))
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// TODO (crbug/1126816): Showoff help results in Launcher Search.
	var subtests = []testData{
		{
			// Rich Omnibox Entities in Launcher Search (crbug/1171390).
			searchKeyword:  "hello in spanish",
			validateAction: uiauto.New(tconn).WaitUntilExists(launcher.SearchResultListItemFinder.NameContaining("Hola")),
		},
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	for _, subtest := range subtests {
		s.Run(ctx, string(subtest.searchKeyword), func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			if err := uiauto.Combine("search in expanded launcher",
				launcher.Open(tconn),
				launcher.Search(tconn, kb, subtest.searchKeyword),
				subtest.validateAction,
			)(ctx); err != nil {
				s.Fatal("Failed to search: ", err)
			}
		})
	}
}
