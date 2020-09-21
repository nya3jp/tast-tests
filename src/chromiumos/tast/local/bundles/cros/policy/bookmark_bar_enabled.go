// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
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

	// Open a keyboard device.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	for _, param := range []struct {
		name               string
		bookmarkPolicy     *policy.BookmarkBarEnabled
		bookmarkBarEnabled bool
	}{
		{
			name:               "enabled",
			bookmarkPolicy:     &policy.BookmarkBarEnabled{Val: true},
			bookmarkBarEnabled: true,
		},
		{
			name:               "disabled",
			bookmarkPolicy:     &policy.BookmarkBarEnabled{Val: false},
			bookmarkBarEnabled: false,
		},
		{
			name:               "unset",
			bookmarkPolicy:     &policy.BookmarkBarEnabled{Stat: policy.StatusUnset},
			bookmarkBarEnabled: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.bookmarkPolicy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			conn, err := cr.NewConn(ctx, "chrome://policy")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Find and click the "Bookmark this tab" button.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get the connection to the test API: ", err)
			}

			bookmarkThisTabButton, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				ClassName: "StarView",
				Role:      ui.RoleTypeButton,
			}, 15*time.Second)
			if err != nil {
				s.Fatal(`Failed to find "Bookmark this tab" button: `, err)
			}
			defer bookmarkThisTabButton.Release(ctx)

			// Click the star button "Bookmark this tab" to add the current tab to bookmarks. The tab should then be visible in the bookmark bar, provided the bar can be displayed.
			if err := bookmarkThisTabButton.LeftClick(ctx); err != nil {
				s.Fatal(`Failed to click "Bookmark this tab" button: `, err)
			}

			// Find and click the "Done" button that appears after clicking on the "Bookmark this tab" button.
			doneButton, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Name: "Done",
				Role: ui.RoleTypeButton,
			}, 15*time.Second)
			if err != nil {
				s.Fatal(`Failed to find "Done" button: `, err)
			}
			defer doneButton.Release(ctx)

			if err := doneButton.LeftClick(ctx); err != nil {
				s.Fatal(`Failed to click "Done" button: `, err)
			}

			// Go to google.com to check whether or not the bookmark exists.
			if err := conn.Navigate(ctx, "https://google.com"); err != nil {
				s.Fatal("Failed to navigate to new URL: ", err)
			}

			params := ui.FindParams{
				Name: "Policies",
				Role: ui.RoleTypeButton,
			}

			bookmarkBarFound := false

			// In the case of unset policies, we need to click "Ctrl+Shift+B" to display the bookmark bar and then wait a bit for it to show up.
			if param.bookmarkPolicy.Stat == policy.StatusUnset {
				if err := keyboard.Accel(ctx, "ctrl+shift+b"); err != nil {
					s.Fatal("Failed to press ctrl + shift + b: ", err)
				}

				if err := ui.WaitUntilExists(ctx, tconn, params, 15*time.Second); err != nil {
					s.Fatal("Waiting for bookmark failed: ", err)
				}

				bookmarkBarFound = true
			} else {
				bookmarkBarFound, err = ui.Exists(ctx, tconn, params)
				if err != nil {
					s.Fatal("Checking for bookmark bar failed: ", err)
				}
			}

			if bookmarkBarFound != param.bookmarkBarEnabled {
				s.Errorf("Unexpected bookmark enabled behavior: got %t; want %t", bookmarkBarFound, param.bookmarkBarEnabled)
			}
		})
	}
}
