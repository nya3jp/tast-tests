// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ManagedBookmarks,
		Desc: "Behavior of ManagedBookmarks policy",
		Contacts: []string{
			"ayaelattar@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func ManagedBookmarks(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			conn, err := cr.NewConn(ctx, "chrome://newtab/")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Name: folder,
				Role: ui.RoleTypePopUpButton,
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				s.Fatal("Could not find top level bookmark folder: ", err)
			}

			bookmarks, err := ui.FindAll(ctx, tconn, ui.FindParams{Role: ui.RoleTypeMenuItem})
			if err != nil {
				s.Fatal("Failed to find bookmarks: ", err)
			}
			defer bookmarks.Release(ctx)

			if len(bookmarks) != len(param.value.Val) {
				s.Errorf("Unexpected number of bookmarks: got %d, expected %d bookmark(s)", len(bookmarks), len(param.value.Val))
			}

			for _, bookmark := range param.value.Val {
				params := ui.FindParams{
					Role: ui.RoleTypeMenuItem,
					Name: bookmark.Name,
				}
				err := ui.WaitUntilExists(ctx, tconn, params, 15*time.Second)
				if err != nil {
					s.Fatal("Could not find bookmark name: ", err)
				}
			}
		})
	}
}
