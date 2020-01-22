// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/policy"
	"chromiumos/tast/local/policy/fakedms"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Disable3DAPIs,
		Desc: "Behavior of the Disable3DAPIs policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.Standard,
	})
}

func Disable3DAPIs(ctx context.Context, s *testing.State) {
	helper := s.PreValue().(*pre.UserPoliciesHelper)

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.Disable3DAPIs
	}{
		{
			name:  "unset",
			value: &policy.Disable3DAPIs{Stat: policy.StatusUnset},
		},
		{
			name:  "false",
			value: &policy.Disable3DAPIs{Val: false},
		},
		{
			name:  "true",
			value: &policy.Disable3DAPIs{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup
			if err := helper.Cleanup(ctx); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Create a policy blob and have the FakeDMS serve it.
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicies([]policy.Policy{param.value})
			if err := helper.UpdatePolicies(ctx, pb); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			conn, err := helper.Chrome.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var isEnabled bool
			if err := conn.Eval(ctx, `(() => {
				var gl;
				try {
					let canvas = document.createElement('canvas');
					gl = canvas.getContext("webgl");
				} catch (e) {
					gl = null;
				}
				return gl !== null
			})()`, &isEnabled); err != nil {
				s.Fatal("Could not get webgl status: ", err)
			}

			expectedDisabled := param.value.Stat != policy.StatusUnset && param.value.Val

			if expectedDisabled && isEnabled {
				s.Error("WebGL not blocked")
			}

			if !expectedDisabled && !isEnabled {
				s.Log("WebGL not available")
			}
		})
	}
}
