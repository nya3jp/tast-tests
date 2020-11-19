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
		Func: ManagedBookmarks,
		Desc: "Behavior of ManagedBookmarks policy",
		Contacts: []string{
			"ayaelattar@google.com", // Test author
			"gabormagda@google.com",
			"alexanderhartl@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func ManagedBookmarks(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	const topLvl = "Policy test folder"

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.ManagedBookmarks
	}{
		{
			name:  "NotSet_NotShown",
			value: &policy.ManagedBookmarks{Stat: policy.StatusUnset},
		},
		{
			name: "SingleBookmark_Shown",
			value: &policy.ManagedBookmarks{Val: []*policy.RefBookmarkType{
				{
					Name:         "Google",
					Url:          "https://google.com/",
					ToplevelName: topLvl,
				},
				{
					Name:         "Youtube",
					Url:          "https://youtube.com/",
					ToplevelName: topLvl,
				},
			},
			},
		},
		{
			name: "MultipleBookmarks_Shown",
			value: &policy.ManagedBookmarks{Val: []*policy.RefBookmarkType{
				{
					Name:         "Google",
					Url:          "https://google.com/",
					ToplevelName: topLvl,
				},
				{
					Name:         "YouTube",
					Url:          "https://youtube.com/",
					ToplevelName: topLvl,
				},
				{
					Name:         "Chromium",
					Url:          "https://chromium.org/",
					ToplevelName: topLvl,
				},
			},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			conn, err := cr.NewConn(ctx, "chrome://newtab/")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			if param.value.Stat != policy.StatusUnset {
				if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
					Name: param.value.Val[0].ToplevelName,
					Role: ui.RoleTypePopUpButton,
				}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
					s.Fatal("Could not find Google bookmark: ", err)
				}

				for _, bookmark := range param.value.Val {

					params := ui.FindParams{
						Role: ui.RoleTypeMenuItem,
						Name: bookmark.Name,
					}
					node, err := ui.FindWithTimeout(ctx, tconn, params, 15*time.Second)
					if err != nil {
						s.Fatal("Finding Automatically click when the cursor stops node failed: ", err)
					}

					defer node.Release(ctx)
				}
			}
		})
	}
}
