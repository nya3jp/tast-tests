// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
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
		Func: SearchSuggestEnabled,
		Desc: "Behavior of SearchSuggestEnabled policy, check if a search suggestions are shown based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func SearchSuggestEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Open a keyboard device.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	for _, param := range []struct {
		name    string
		enabled bool                         // enabled is the expected enabled state of the virtual keyboard.
		policy  *policy.SearchSuggestEnabled // policy is the policy we test.
	}{
		{
			name:    "unset",
			enabled: true,
			policy:  &policy.SearchSuggestEnabled{Stat: policy.StatusUnset},
		},
		{
			name:    "disabled",
			enabled: false,
			policy:  &policy.SearchSuggestEnabled{Val: false},
		},
		{
			name:    "enabled",
			enabled: true,
			policy:  &policy.SearchSuggestEnabled{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Try to open a tab.
			if err := keyboard.Accel(ctx, "ctrl+n"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Click the address and search bar.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "Address and search bar",
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Failed to click address bar: ", err)
			}

			// Type something so suggestions pop up.
			if err := keyboard.Type(ctx, "google"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
				s.Fatal("Failed to wait for location change: ", err)
			}

			// Get the omnibox popup node.
			// This node collects all the suggestions.
			omniPopup, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "OmniboxPopupContentsView"}, 10*time.Second)
			if err != nil {
				s.Fatal("Failed to find omnibox popup: ", err)
			}
			defer omniPopup.Release(ctx)

			// There is one children for each suggestion in the omnibox popup.
			omniResults, err := omniPopup.Children(ctx)
			if err != nil {
				s.Fatal("Failed to get omnibox results: ", err)
			}
			defer omniResults.Release(ctx)

			suggest := false
			for _, result := range omniResults {
				// Every result has one child with the suggestion in their name.
				view, err := result.Descendant(ctx, ui.FindParams{})
				if err != nil {
					s.Fatal("Failed to get omnibox result view: ", err)
				}
				defer view.Release(ctx)

				if strings.Contains(view.Name, "search suggestion") {
					suggest = true
					break
				}
			}

			if suggest != param.enabled {
				s.Errorf("Unexpected existence of search suggestions: got %t; want %t", suggest, param.enabled)
			}
		})
	}
}
