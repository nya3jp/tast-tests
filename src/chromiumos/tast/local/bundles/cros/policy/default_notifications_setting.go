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
		Func: DefaultNotificationsSetting,
		Desc: "Behavior of DefaultNotificationsSetting policy, checking if notifications are blocked/allowed after setting the policy",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func DefaultNotificationsSetting(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, param := range []struct {
		name          string
		expectedValue string
		policy        *policy.DefaultNotificationsSetting // policy is the policy we test.
	}{
		{
			name:          "unset",
			expectedValue: "default",
			policy:        &policy.DefaultNotificationsSetting{Stat: policy.StatusUnset},
		},
		{
			name:          "allow",
			expectedValue: "granted",
			policy:        &policy.DefaultNotificationsSetting{Val: 1},
		},
		{
			name:          "block",
			expectedValue: "denied",
			policy:        &policy.DefaultNotificationsSetting{Val: 2},
		},
		{
			name:          "ask",
			expectedValue: "default",
			policy:        &policy.DefaultNotificationsSetting{Val: 3},
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

			conn, err := cr.NewConn(ctx, "chrome://policy")
			if err != nil {
				s.Fatal("Creating renderer failed: ", err)
			}
			defer conn.Close()

			var permission string
			if err := conn.Eval(ctx, `Notification.permission`, &permission); err != nil {
				s.Fatal("Failed to request notification permission: ", err)
			}

			if permission != param.expectedValue {
				s.Errorf("Failed to verify test %q, expected: %q and actual: %q", param.name, param.expectedValue, permission)
			}
		})
	}
}
