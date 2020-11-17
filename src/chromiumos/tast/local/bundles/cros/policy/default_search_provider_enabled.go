// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultSearchProviderEnabled,
		Desc: "Behavior of DefaultSearchProviderEnabled policy",
		Contacts: []string{
			"anastasiian@chromium.org",
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func DefaultSearchProviderEnabled(ctx context.Context, s *testing.State) {
	const (
		defaultSearchEngine = "google.com" // search engine checked in the test
		pageLoadTime        = time.Second  // time reserved for page load / google search.
	)

	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// // Connect to Test API to use it with the UI library.
	// tconn, err := cr.TestAPIConn(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to create Test API connection: ", err)
	// }

	// // Set up keyboard.
	// kb, err := input.Keyboard(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to get keyboard: ", err)
	// }
	// defer kb.Close()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// enabled is the expected enabled state of the policy.
		enabled bool
		// value is the policy value.
		value *policy.DefaultSearchProviderEnabled
	}{
		{
			name:    "true",
			enabled: true,
			value:   &policy.DefaultSearchProviderEnabled{Val: true},
		},
		{
			name:    "false",
			enabled: false,
			value:   &policy.DefaultSearchProviderEnabled{Val: false},
		},
		{
			name:    "unset",
			enabled: true,
			value:   &policy.DefaultSearchProviderEnabled{Stat: policy.StatusUnset},
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

			// Open an empty page.
			conn, err := cr.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// // Click the address and search bar.
			// if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
			// 	Role: ui.RoleTypeTextField,
			// 	Name: "Address and search bar",
			// }, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			// 	s.Fatal("Failed to click address bar: ", err)
			// }

			// // Type something.
			// if err := kb.Type(ctx, "abc\n"); err != nil {
			// 	s.Fatal("Failed to write events: ", err)
			// }

			// // Check whether the address bar includes search engine url.
			// var searchEngineURLIsInOmnibox bool
			// if err := conn.Eval(ctx, fmt.Sprintf("location.href.includes(%q)", defaultSearchEngine), &searchEngineURLIsInOmnibox); err != nil {
			// 	s.Fatal("Failed to execute JS expression: ", err)
			// }

			// if param.enabled != searchEngineURLIsInOmnibox {
			// 	s.Fatalf("Unexpected usage of search engine: got %t; want %t", searchEngineURLIsInOmnibox, param.enabled)
			// }
		})
	}
}
