// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PolicyRefreshRate,
		Desc: "Behavior of PolicyRefreshRate policy",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// PolicyRefreshRate tests the PolicyRefreshRate policy.
func PolicyRefreshRate(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name           string
		expected_value string                    // expected_value is the value that should be set.
		value          *policy.PolicyRefreshRate // value is the value of the policy.
	}{
		{
			name:           "min_allowed_value",
			expected_value: " 30 mins",
			value:          &policy.PolicyRefreshRate{Val: 1800000},
		},
		{
			name:           "max_allowed_value",
			expected_value: " 1 day",
			value:          &policy.PolicyRefreshRate{Val: 86400000},
		},
		{
			name:           "below_min_allowed_value",
			expected_value: " 30 mins",
			value:          &policy.PolicyRefreshRate{Val: 100},
		},
		{
			name:           "above_max_allowed_value",
			expected_value: " 1 day",
			value:          &policy.PolicyRefreshRate{Val: 186400000},
		},
		{
			name:           "unset",
			expected_value: " 3 hours",
			value:          &policy.PolicyRefreshRate{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the policy page.
			conn, err := cr.NewConn(ctx, "chrome://policy")
			if err != nil {
				s.Fatal("Failed to connect to the policy page: ", err)
			}
			defer conn.Close()

			var refresh_value string
			if err := conn.Eval(ctx, `document.getElementById("status-box-container").getElementsByClassName("refresh-interval")[1].innerText`, &refresh_value); err != nil {
				s.Fatal("Could not read policy page: ", err)
			}
			// Check the refresh value.
			if refresh_value != param.expected_value {
				s.Errorf("Unexpected refresh value: got %v; want %v", refresh_value, param.expected_value)
			}

		})
	}
}
