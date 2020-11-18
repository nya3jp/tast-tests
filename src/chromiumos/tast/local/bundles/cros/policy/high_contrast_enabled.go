// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HighContrastEnabled,
		Desc: "Behavior of HighContrastEnabled policy",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func HighContrastEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name    string                      // name is the subtest name.
		enabled bool                        // enabled is the expected enabled state of the policy.
		value   *policy.HighContrastEnabled // value is the policy value.
	}{
		{
			name:    "true",
			enabled: true,
			value:   &policy.HighContrastEnabled{Val: true},
		},
		{
			name:    "false",
			enabled: false,
			value:   &policy.HighContrastEnabled{Val: false},
		},
		{
			name:    "unset",
			enabled: false,
			value:   &policy.HighContrastEnabled{Stat: policy.StatusUnset},
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
			var highContrastValue bool
			script :=
				`new Promise((resolve, reject) => {
					chrome.accessibilityFeatures.highContrast.get({}, (details) => resolve(details.value));
				})`
			if err := tconn.EvalPromise(ctx, script, &highContrastValue); err != nil {
				s.Fatal("Failed to eval: ", err)
			}

			if param.enabled != highContrastValue {
				s.Fatalf("Unexpected value of high contrast: got %t; want %t", highContrastValue, param.enabled)
			}
		})
	}
}
