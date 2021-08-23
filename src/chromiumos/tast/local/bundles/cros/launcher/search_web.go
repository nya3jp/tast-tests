// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"regexp"
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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchWeb,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "App Launcher Search: Web",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "drivefs", "chrome_internal"},
		Fixture:      "chromeLoggedIn",
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

	query := "web browser"

	// The expected result will not be an app, so launcher.SearchAndLaunchWithQuery and other similar functions do not work.
	if err := uiauto.Combine(fmt.Sprintf("search %q in launcher", query),
		launcher.Open(tconn),
		launcher.Search(tconn, kb, query),
	)(ctx); err != nil {
		s.Fatalf("Failed to search %s in launcher: %v", query, err)
	}

	resultFinder := launcher.SearchResultListItemFinder.Name(query + ", Google Search")
	ui := uiauto.New(tconn)

	if err := ui.LeftClick(resultFinder)(ctx); err != nil {
		s.Fatalf("Failed to left click %s in launcher: %v", query, err)
	}
	defer ash.CloseAllWindows(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "launched_result_ui_dump")

	browserRootFinder := nodewith.Role(role.Window).HasClass("BrowserRootView")
	verifyNode := browserRootFinder.NameRegex(regexp.MustCompile(fmt.Sprintf("^%s - Google .* - Google Chrome - .*", query)))

	if err := uiauto.New(tconn).WaitUntilExists(verifyNode)(ctx); err != nil {
		s.Fatal("Failed to verify search result: ", err)
	}
}
