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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchWeb,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "App Launcher Search: Web",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "drivefs", "chrome_internal"},
		Fixture:      "chromeLoggedIn",
		Timeout:      time.Minute,
	})
}

// SearchWeb tests that App Launcher Search: Web.
func SearchWeb(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "search_web")

	keyword := "web browser"
	SearchResult := launcher.SearchResultListItemFinder.Name(keyword + ", Google Search")

	if err := launcher.SearchAndLeftClick(ctx, tconn, kb, keyword, SearchResult); err != nil {
		s.Fatal("Failed to search and left click in launcher: ", err)
	}
	defer ash.CloseAllWindows(cleanupCtx, tconn)

	if err := uiauto.New(tconn).WaitUntilExists(nodewith.NameContaining(keyword).HasClass("BrowserFrame"))(ctx); err != nil {
		s.Fatalf("Failed to wait %s open: %v", keyword, err)
	}
}
