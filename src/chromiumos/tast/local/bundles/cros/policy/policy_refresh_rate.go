// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PolicyRefreshRate,
		Desc: "Behavior of PolicyRefreshRate policy",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// PolicyRefreshRate tests the PolicyRefreshRate policy.
func PolicyRefreshRate(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, param := range []struct {
		name          string
		expectedValue string                    // expectedValue is the value that should be set.
		value         *policy.PolicyRefreshRate // value is the value of the policy.
	}{
		{
			name:          "min_allowed_value",
			expectedValue: " 30 mins",
			value:         &policy.PolicyRefreshRate{Val: 1800000},
		},
		{
			name:          "max_allowed_value",
			expectedValue: " 1 day",
			value:         &policy.PolicyRefreshRate{Val: 86400000},
		},
		{
			name:          "below_min_allowed_value",
			expectedValue: " 30 mins",
			value:         &policy.PolicyRefreshRate{Val: 100},
		},
		{
			name:          "above_max_allowed_value",
			expectedValue: " 1 day",
			value:         &policy.PolicyRefreshRate{Val: 186400000},
		},
		{
			name:          "unset",
			expectedValue: " 3 hours",
			value:         &policy.PolicyRefreshRate{Stat: policy.StatusUnset},
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

			// Open the policy page.
			conn, err := cr.NewConn(ctx, "chrome://policy")
			if err != nil {
				s.Fatal("Failed to connect to the policy page: ", err)
			}
			defer conn.Close()

			var refreshValue string
			if err := conn.Eval(ctx, `document.querySelector("#status-box-container .refresh-interval").innerText`, &refreshValue); err != nil {
				s.Fatal("Could not read policy page: ", err)
			}
			// Check the refresh value.
			if refreshValue != param.expectedValue {
				s.Errorf("Unexpected refresh value: got %v; want %v", refreshValue, param.expectedValue)
			}

		})
	}
}
