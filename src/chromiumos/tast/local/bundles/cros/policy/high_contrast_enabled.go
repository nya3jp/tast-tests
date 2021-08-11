// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HighContrastEnabled,
		Desc: "Behavior of HighContrastEnabled policy",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func HighContrastEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name    string                      // name is the subtest name.
		enabled bool                        // enabled is the expected enabled state of the policy.
		policy  *policy.HighContrastEnabled // policy is the value of 'HighContrastEnabled' policy, setting the policy to true keeps High-contrast mode on.
	}{
		{
			name:    "true",
			enabled: true,
			policy:  &policy.HighContrastEnabled{Val: true},
		},
		{
			name:    "false",
			enabled: false,
			policy:  &policy.HighContrastEnabled{Val: false},
		},
		{
			name:    "unset",
			enabled: false,
			policy:  &policy.HighContrastEnabled{Stat: policy.StatusUnset},
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

			// Run actual test.
			var highContrastValue bool
			script := `(async () => {
				let result = await tast.promisify(tast.bind(chrome.accessibilityFeatures.highContrast, "get"))({});
				return result.value;
			  })()`
			if err := tconn.Eval(ctx, script, &highContrastValue); err != nil {
				s.Fatal("Failed to eval: ", err)
			}

			if highContrastValue != param.enabled {
				s.Fatalf("Unexpected value of chrome.accessibilityFeatures.highContrast: got %t; want %t", highContrastValue, param.enabled)
			}
		})
	}
}
