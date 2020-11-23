// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BookmarkBarEnabled,
		Desc: "Test the behavior of BookmarkBarEnabled policy: check if bookmark bar is shown based on the value of the policy",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Pre:          pre.User,
	})
}

// BookmarkBarEnabled validates the UI behavior of the different
// states the policy introduces. When enabled the bookmark bar
// appears with list of bookmarks otherwise it should not appear.
func BookmarkBarEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	const bookmarkName = "Policies"
	// Bookmark the URL: chrome://policy.
	func() {
		conn, err := cr.NewConn(ctx, "chrome://policy")
		if err != nil {
			s.Fatal("Failed to connect to chrome: ", err)
		}
		defer conn.Close()

		// Bookmark the current URL by clicking on the star button.
		if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
			Role: ui.RoleTypeButton,
			Name: "Bookmark this tab",
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Failed to find and click star button: ", err)
		}

		// Star button clicked. Adding bookmark dialog should be shown.
		// Find and click the Name field in the dialog to specify a name for the bookmark.
		if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
			Role: ui.RoleTypeTextField,
			Name: "Bookmark name",
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			s.Fatal("Failed to click address bar: ", err)
		}

		// Set up keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to get keyboard: ", err)
		}
		defer kb.Close()

		// Select the existing text in the Name field so that it can be deleted by pressing backspace in the next step.
		if err := kb.Accel(ctx, "Ctrl+a"); err != nil {
			s.Fatal("Failed to select bookmark name: ", err)
		}

		// Delete the existing text and type a name for the current bookmark.
		if err := kb.Type(ctx, "\b"+bookmarkName); err != nil {
			s.Fatal("Failed to type bookmark name: ", err)
		}

		// Click the "Done" button on the dialog.
		if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
			Role: ui.RoleTypeButton,
			Name: "Done",
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Failed to find and click done button: ", err)
		}
	}()

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
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open a new URL to check whether bookmark bar is shown.
			vconn, err := cr.NewConn(ctx, "chrome://version")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer vconn.Close()

			// Confirm whether bookmark bar is shown with the bookmarked URL.
			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: bookmarkName,
			}, param.wantBookmarbar, 10*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the bookmark bar: ", err)
			}
		})
	}
}
