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

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.ManagedBookmarks
	}{
		// {
		// 	name:  "NotSet_NotShown",
		// 	value: &policy.ManagedBookmarks{Val: policy.StatusUnset}
		// },
		{
			name: "SingleBookmark_Shown",
			value: &policy.ManagedBookmarks{Val: []*policy.RefBookmarkType{
				{
					Name:         "Google",
					Url:          "https://google.com/",
					Children:     []*policy.RefBookmarkType{},
					ToplevelName: "Test",
				},
				{
					Name:         "Youtube",
					Url:          "https://youtube.com/",
					Children:     []*policy.RefBookmarkType{},
					ToplevelName: "Test",
				},
			},
			},
		},
		// {
		// 	name:  "MultipleBookmarks_Shown",
		// 	value: &policy.ManagedBookmarks{Val:
		// 		[]Bookmark{
		// 			{
		// 				name: "Google",
		// 				url: "https://google.com/"
		// 			},
		// 			{
		// 				name: "YouTube",
		// 				url: "https://youtube.com/"
		// 			},
		// 			{
		// 				name: "Chromium",
		// 				url: "https://chromium.org/"
		// 			},
		// 		},
		// 	}
		// },
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
			conn, err := cr.NewConn(ctx, "chrome://bookmarks")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			tbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Name: "Google",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Could not find Google bookmark: ", err)
			}
			defer tbNode.Release(ctx)

		})
	}
}
