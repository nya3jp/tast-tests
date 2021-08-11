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
		Func: LargeCursorEnabled,
		Desc: "Behavior of LargeCursorEnabled policy",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func LargeCursorEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name    string                     // name is the subtest name.
		enabled bool                       // enabled is the expected enabled state of the policy.
		value   *policy.LargeCursorEnabled // value is the policy value.
	}{
		{
			name:    "true",
			enabled: true,
			value:   &policy.LargeCursorEnabled{Val: true},
		},
		{
			name:    "false",
			enabled: false,
			value:   &policy.LargeCursorEnabled{Val: false},
		},
		{
			name:    "unset",
			enabled: false,
			value:   &policy.LargeCursorEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			var largeCursorValue bool
			script := `(async () => {
				let result = await tast.promisify(tast.bind(chrome.accessibilityFeatures.largeCursor, "get"))({});
				return result.value;
			  })()`
			if err := tconn.Eval(ctx, script, &largeCursorValue); err != nil {
				s.Fatal("Failed to retrieve largeCursor enabled value: ", err)
			}

			if param.enabled != largeCursorValue {
				s.Fatalf("Unexpected value of chrome.accessibilityFeatures.largeCursor: got %t; want %t", largeCursorValue, param.enabled)
			}
		})
	}
}
