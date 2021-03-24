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
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SearchInHelpApp,
		Desc: "Help app local search service works",
		Contacts: []string{
			"callistus@chromium.org", // test author.
			"showoff-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
			},
		},
	})
}

// SearchInHelpApp verifies the local search service is being used by the app,
// and results navigate as expected.
func SearchInHelpApp(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("HelpAppSearchServiceIntegration"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)
	helpCtx := helpapp.NewContext(cr, tconn)

	if err := uiauto.Combine("launch Help app and navigate to search page",
		helpCtx.Launch(),
		ui.LeftClick(helpapp.SearchTabFinder),
	)(ctx); err != nil {
		s.Fatal("Failed to launch help app or navigate to search page: ", err)
	}

	// Establish a Chrome connection to the Help app and wait for it to finish
	// initializing the local search service.
	helpAppConn, err := cr.NewConnForTarget(ctx,
		chrome.MatchTargetURL("chrome://help-app/"))
	if err != nil {
		s.Fatal("Failed to establish connection to help app")
	}
	defer helpAppConn.Close()

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		responseStatus := -1
		err := helpAppConn.Eval(ctx,
			`(async () => {
				return (await indexRemote.find(toString16('foo'))).status;
			})()`, &responseStatus)
		if err != nil {
			return errors.Wrap(err, "failed to run javascript to check LSS status")
		}

		// Status 1 corresponds to kSuccess.
		// https://source.chromium.org/chromium/chromium/src/+/HEAD:chromeos/components/local_search_service/mojom/types.mojom;l=68;drc=378c706113a7a8573a184d60e1bd67d704644251
		if responseStatus != 1 {
			return errors.Wrap(err, "response status not equal to kSuccess")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to initialize local search service")
	}

	// Search for "halp". If local search service is used, it should return
	// results for "help".
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer keyboard.Close()

	searchResultFinder := nodewith.ClassName("search-result selected").Role(role.ListItem).Ancestor(helpapp.RootFinder)

	if err := uiauto.Combine("type keyword to search and validate result",
		keyboard.TypeAction("halp"),
		ui.WaitUntilExists(searchResultFinder),
	)(ctx); err != nil {
		s.Error("Failed to search in Help app: ", err)
	}
}
