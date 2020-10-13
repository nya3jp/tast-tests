// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
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

			// Poll until search suggestions are found.
			paramsS := ui.FindParams{
				ClassName: "OmniboxResultView",
			}
			var errNoSearchSuggestions = errors.New("failed to find search suggestions")
			hasSuggestions := false
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				nodes, err := ui.FindAll(ctx, tconn, paramsS)
				if errors.Is(err, context.DeadlineExceeded) {
					return err
				} else if err != nil {
					return testing.PollBreak(err)
				}
				defer nodes.Release(ctx)

				// Check if we have search suggestions.
				for _, node := range nodes {
					if strings.Contains(node.Name, "search suggestion") {
						hasSuggestions = true
						return nil
					}
				}

				return errNoSearchSuggestions
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil && !errors.Is(err, errNoSearchSuggestions) {
				s.Fatal("Failed to retrieve the suggestions: ", err)
			}

			if hasSuggestions != param.enabled {
				s.Errorf("Unexpected existence of search suggestions: got %t; want %t", hasSuggestions, param.enabled)
			}
		})
	}
}
