// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BookmarkBarEnabled,
		Desc: "Behavior of BookmarkBarEnabled policy",
		Contacts: []string{
			"muhamedp@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func BookmarkBarEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.BookmarkBarEnabled
	}{
		{
			name:  "true",
			value: &policy.BookmarkBarEnabled{Val: true},
		},
		{
			name:  "false",
			value: &policy.BookmarkBarEnabled{Val: false},
		},
		{
			name:  "unset",
			value: &policy.BookmarkBarEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			conn, err := cr.NewConn(ctx, "chrome://policy")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Find and click the "Bookmark this tab" button.
			params := ui.FindParams{
				ClassName: "StarView",
				Role:      ui.RoleTypeButton,
			}

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get the connection to the test API: ", err)
			}

			bookmarkThisButton, err := ui.Find(ctx, tconn, params)
			if err != nil {
				s.Fatal("Failed to find bookmark button: ", err)
			}

			// Click the star button "Bookmark this tab" to add the current tab to bookmarks. The tab should then be visible in the bookmark bar, provided the bar can be displayed.
			if err := bookmarkThisButton.LeftClick(ctx); err != nil {
				s.Fatal(`Failed to click "Bookmark this tab" button: `, err)
			}

			// Find and click the "Done" button that appears after clicking on the "Bookmark this tab" button.
			params = ui.FindParams{
				Name: "Done",
				Role: ui.RoleTypeButton,
			}

			doneButton, err := ui.Find(ctx, tconn, params)
			if err != nil {
				s.Fatal(`Failed to find "Done" button: `, err)
			}

			if err := doneButton.LeftClick(ctx); err != nil {
				s.Fatal(`Failed to click "Done" button: `, err)
			}

			// Go to google.com to check whether or not the bookmark exists.
			if err := conn.Navigate(ctx, "https://google.com"); err != nil {
				s.Fatal("Failed to navigate to new URL: ", err)
			}

			params = ui.FindParams{
				Name: "Policies",
				Role: ui.RoleTypeButton,
			}

			bookmarkBarFound := false

			// In the case of unset policies, we need to click "Ctrl+Shift+B" to display the bookmark bar and then wait a bit for it to show up.
			if param.value.Stat == policy.StatusUnset {
				if err := tconn.Call(ctx, nil, `async () => {
					let accelerator = {keyCode: 'b', shift: true, control: true, alt: false, search: false, pressed: true};
					await tast.promisify(chrome.autotestPrivate.activateAccelerator)(accelerator);
					accelerator.pressed = false;
					await tast.promisify(chrome.autotestPrivate.activateAccelerator)(accelerator);
				}`); err != nil {
					s.Fatal("Failed to toggle bookmark bar: ", err)
				}

				err := ui.WaitUntilExists(ctx, tconn, params, 5*time.Second)
				if err != nil {
					s.Fatal("Waiting for bookmark failed: ", err)
				}

				bookmarkBarFound = true
			} else {
				bookmarkBarFound, err = ui.Exists(ctx, tconn, params)
				if err != nil {
					s.Fatal("Checking for bookmark bar failed: ", err)
				}
			}

			expectedBookmarkBarEnabled := param.value.Stat == policy.StatusUnset || param.value.Val

			if bookmarkBarFound != expectedBookmarkBarEnabled {
				s.Errorf("Unexpected enabled behavior: got %t; want %t", bookmarkBarFound, expectedBookmarkBarEnabled)
			}
		})
	}
}
