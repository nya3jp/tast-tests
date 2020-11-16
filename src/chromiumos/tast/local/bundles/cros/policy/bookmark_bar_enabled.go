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
		Attr:         []string{"group:mainline", "informational"},
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

		// Clicking on the star button opens a dialog. Click on the done button on it.
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
			if err := ui.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Policies",
			}, param.wantBookmarbar, 10*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the bookmark bar: ", err)
			}
		})
	}
}
