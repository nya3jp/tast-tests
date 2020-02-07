// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/policyutil"
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
		Pre:          pre.User,
	})
}

func Disable3DAPIs(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

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
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			conn, err := cr.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var enabled bool
			if err := conn.Eval(ctx, `(() => {
			  try {
			    let canvas = document.createElement('canvas');
			    return !!canvas.getContext('webgl');
			  } catch (e) {
			    return false;
			  }
			})()`, &enabled); err != nil {
				s.Fatal("Could not get webgl status: ", err)
			}

			expectEnabled := param.value.Stat == policy.StatusUnset || param.value.Val == false

			if !expectEnabled && enabled {
				s.Error("WebGL not blocked")
			}

			// WebGL may not be available on all devices.
			if expectEnabled && !enabled {
				s.Log("WebGL not available")
			}
		})
	}
}
