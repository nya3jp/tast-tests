// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ManagedBookmarks,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of ManagedBookmarks policy",
		Contacts: []string{
			"sergiyb@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ManagedBookmarks{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func ManagedBookmarks(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	const folder = "Policy test folder"

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.ManagedBookmarks
	}{
		{
			name: "single",
			value: &policy.ManagedBookmarks{Val: []*policy.RefBookmarkType{
				{
					Name:         "Google",
					Url:          "https://google.com/",
					ToplevelName: folder,
				},
			},
			},
		},
		{
			name: "multiple",
			value: &policy.ManagedBookmarks{Val: []*policy.RefBookmarkType{
				{
					Name:         "Google",
					Url:          "https://google.com/",
					ToplevelName: folder,
				},
				{
					Name:         "YouTube",
					Url:          "https://youtube.com/",
					ToplevelName: folder,
				},
				{
					Name:         "Chromium",
					Url:          "https://chromium.org/",
					ToplevelName: folder,
				},
			},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to set up browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Run actual test.
			conn, err := br.NewConn(ctx, "chrome://newtab/")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
			bookmarkMenuItemFinder := nodewith.Role(role.MenuItem)
			if err := uiauto.Combine("click on folder and wait for menu to appear",
				ui.LeftClick(nodewith.Name(folder).Role(role.PopUpButton)),
				ui.WaitUntilExists(bookmarkMenuItemFinder.First()),
			)(ctx); err != nil {
				s.Fatal("Could not find top level bookmark folder: ", err)
			}

			bookmarks, err := ui.NodesInfo(ctx, bookmarkMenuItemFinder)
			if err != nil {
				s.Fatal("Failed to find bookmarks: ", err)
			}

			if len(bookmarks) != len(param.value.Val) {
				s.Errorf("Unexpected number of bookmarks: got %d, expected %d bookmark(s)", len(bookmarks), len(param.value.Val))
			}

			for _, bookmark := range param.value.Val {
				if err := ui.WaitUntilExists(nodewith.Role(role.MenuItem).Name(bookmark.Name))(ctx); err != nil {
					s.Fatal("Could not find bookmark name: ", err)
				}
			}
		})
	}
}
