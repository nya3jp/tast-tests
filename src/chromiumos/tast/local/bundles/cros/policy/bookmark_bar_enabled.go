// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BookmarkBarEnabled,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test the behavior of BookmarkBarEnabled policy: check if bookmark bar is shown based on the value of the policy",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{lacrosfixt.LacrosDeployedBinary},
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
	})
}

const (
	bookmarkName = "Policies"
	policiesURL  = "chrome://policy"
)

// BookmarkBarEnabled validates the UI behavior of the different
// states the policy introduces. When enabled the bookmark bar
// appears with list of bookmarks otherwise it should not appear.
func BookmarkBarEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Launch a persistent browser.
	br, closeBrowser, err := browserfixt.SetUpPersistent(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to set up persistent browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open the policy page in order to bookmark it.
	conn, err := br.NewConn(ctx, policiesURL)
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	// Connect to Test API to use it with the UI library.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)

	// TODO(chromium:1280200) Wait for browser window.

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Bookmark the current URL using keyboard shortcut.
	if err := kb.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to open bookmark added popup: ", err)
	}

	// Adding bookmark dialog should be shown.
	// Find and click the name field in the dialog to specify the bookmark name.
	bookmarkNameField := nodewith.Name("Bookmark name").Role(role.TextField)
	if err := uiauto.Combine("find and click the bookmark name text field",
		ui.WaitUntilExists(bookmarkNameField),
		ui.LeftClick(bookmarkNameField),
	)(ctx); err != nil {
		s.Fatal("Failed to find and click the bookmark name text field: ", err)
	}

	// Select the existing text in the bookmark name field so that it can be deleted by pressing
	// backspace in the next step.
	if err := kb.Accel(ctx, "Ctrl+a"); err != nil {
		s.Fatal("Failed to select bookmark name: ", err)
	}

	// Delete the existing text and type a name for the current bookmark.
	if err := kb.Type(ctx, "\b"+bookmarkName); err != nil {
		s.Fatal("Failed to type bookmark name: ", err)
	}

	// Click the "Done" button on the dialog.
	if err := ui.LeftClick(nodewith.Name("Done").Role(role.Button).First())(ctx); err != nil {
		s.Fatal("Failed to click the add bookmark done button: ", err)
	}

	for _, param := range []struct {
		name           string                     // name is the subtest name.
		wantBookmarbar bool                       // wantBookmarbar is the expected existence of the bookmark bar.
		policy         *policy.BookmarkBarEnabled // policy is the policy we test.
	}{
		{
			name:           "unset",
			wantBookmarbar: false,
			policy:         &policy.BookmarkBarEnabled{Stat: policy.StatusUnset},
		},
		{
			name:           "disabled",
			wantBookmarbar: false,
			policy:         &policy.BookmarkBarEnabled{Val: false},
		},
		{
			name:           "enabled",
			wantBookmarbar: true,
			policy:         &policy.BookmarkBarEnabled{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			br, closeBrowser, err := browserfixt.SetUpPersistent(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to set up persistent browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Open a new URL to check whether bookmark bar is shown.
			vconn, err := br.NewConn(ctx, "chrome://version")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer vconn.Close()

			// Confirm whether bookmark bar is shown with the bookmarked URL.
			// TODO(crbug.com/1236546): Replace this with a helper function to check the existence of a UI node.
			bookmarkedButton := nodewith.Name(bookmarkName).Role(role.Button).First()
			if err = ui.WaitUntilExists(bookmarkedButton)(ctx); err != nil {
				if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
					s.Fatal("Failed to wait for the bookmark bar: ", err)
				}
				if param.wantBookmarbar {
					s.Error("Bookmark bar with bookmarked URL not found: ", err)
				}
			} else if !param.wantBookmarbar {
				s.Error("Unexpected bookmark bar with bookmarked URL found: ", err)
			}
		})
	}
}
