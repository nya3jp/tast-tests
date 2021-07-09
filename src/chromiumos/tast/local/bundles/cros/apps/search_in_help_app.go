// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SearchInHelpApp,
		Desc: "Help app local search service works",
		Contacts: []string{
			"callistus@chromium.org", // test author.
			"showoff-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "chromeLoggedInForEA",
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

// SearchInHelpApp verifies the local search service is being used by the app,
// and results navigate as expected.
func SearchInHelpApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)
	helpCtx := helpapp.NewContext(cr, tconn)

	if err := uiauto.Combine("launch Help app and navigate to search page",
		helpCtx.Launch(),
		helpCtx.NavigateToSearchPage(),
	)(ctx); err != nil {
		s.Fatal("Failed to launch help app or navigate to search page: ", err)
	}

	// Establish a Chrome connection to the Help app trusted frame and wait for it to finish
	// initializing the local search service.
	trustedHelpAppConn, err := helpCtx.TrustedUIConn(ctx)
	if err != nil {
		s.Fatal("Failed to establish connection to help app trusted frame")
	}
	defer trustedHelpAppConn.Close()

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		responseStatus := -1
		err := trustedHelpAppConn.Eval(ctx,
			`(async () => {
				const toString16 = (s) => ({
					data: Array.from(s, c => c.charCodeAt())
				});
				const indexRemote = chromeos.localSearchService
					.mojom.Index.getRemote();
				return (await indexRemote.find(toString16('foo'))).status;
			})()`, &responseStatus)
		if err != nil {
			return errors.Wrap(err, "failed to run javascript to check LSS status")
		}

		// Status 1 corresponds to kSuccess.
		// https://source.chromium.org/chromium/chromium/src/+/HEAD:chromeos/components/local_search_service/mojom/types.mojom;l=68;drc=378c706113a7a8573a184d60e1bd67d704644251
		if responseStatus != 1 {
			return errors.Wrapf(err, "response status does not equal to kSuccess, want 1, got %d", responseStatus)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to initialize local search service: ", err)
	}

	// Search for "halp". If local search service is used, it should return
	// results for "help".
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer keyboard.Close()

	firstSearchResultContainer := nodewith.ClassName("search-result selected").Role(role.ListItem).Ancestor(helpapp.RootFinder)

	const searchKeyword = "halp"
	expectedResultFinder := nodewith.Role(role.Link).NameContaining("help").Ancestor(firstSearchResultContainer)

	if err := uiauto.Combine("type keyword to search and validate result",
		helpCtx.ClickSearchInputAndWaitForActive(),
		keyboard.TypeAction(searchKeyword),
		ui.WaitUntilExists(expectedResultFinder),
	)(ctx); err != nil {
		s.Error("Failed to search in Help app: ", err)
	}
}
