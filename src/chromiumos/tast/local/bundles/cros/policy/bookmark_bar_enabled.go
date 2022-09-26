// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
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
		Func:         BookmarkBarEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test the behavior of BookmarkBarEnabled policy: check if bookmark bar is shown based on the value of the policy",
		Contacts: []string{
			"snijhara@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ManagedBookmarks{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.BookmarkBarEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// BookmarkBarEnabled validates the UI behavior of the different
// states the policy introduces. When enabled the bookmark bar
// appears with list of bookmarks otherwise it should not appear.
func BookmarkBarEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)

	const folderName = "MyFolder12345"
	var managedBookmarks = &policy.ManagedBookmarks{Val: []*policy.RefBookmarkType{
		{
			ToplevelName: folderName,
			Name:         "Google",
			Url:          "https://google.com/",
		},
	}}

	for _, param := range []struct {
		name            string                     // name is the subtest name.
		wantBookmarkBar bool                       // wantBookmarkBar is the expected existence of the bookmark bar.
		policy          *policy.BookmarkBarEnabled // policy is the policy we test.
	}{
		{
			name:            "unset",
			wantBookmarkBar: false,
			policy:          &policy.BookmarkBarEnabled{Stat: policy.StatusUnset},
		},
		{
			name:            "disabled",
			wantBookmarkBar: false,
			policy:          &policy.BookmarkBarEnabled{Val: false},
		},
		{
			name:            "enabled",
			wantBookmarkBar: true,
			policy:          &policy.BookmarkBarEnabled{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{managedBookmarks, param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to set up browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open a new URL to check whether bookmark bar is shown.
			vconn, err := br.NewConn(ctx, "chrome://version")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer vconn.Close()

			// Confirm whether bookmark bar is shown with the bookmarked URL.
			// TODO(crbug.com/1236546): Replace this with a helper function to check the existence of a UI node.
			folderButton := nodewith.Name(folderName).Role(role.PopUpButton).First()
			if err = ui.WaitUntilExists(folderButton)(ctx); err != nil {
				if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
					s.Fatal("Failed to wait for the bookmark bar: ", err)
				}
				if param.wantBookmarkBar {
					s.Error("Bookmark bar with folder not found: ", err)
				}
			} else if !param.wantBookmarkBar {
				s.Error("Unexpected button in bookmark bar found: ", err)
			}
		})
	}
}
