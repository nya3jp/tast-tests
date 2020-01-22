// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
	})
}

func Disable3DAPIs(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	pb := fakedms.NewPolicyBlob()
	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Set up Chrome Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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
			// Close windows.
			windows, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get windows: ", err)
			}

			for _, window := range windows {
				if err := window.CloseWindow(ctx, tconn); err != nil {
					s.Fatal("Failed to close window: ", err)
				}
			}

			// Create a policy blob and have the FakeDMS serve it.
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicies([]policy.Policy{param.value})
			if err := fdms.WritePolicyBlob(pb); err != nil {
				s.Fatal("Failed to write policies to FakeDMS: ", err)
			}

			// Refresh policies.
			if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)();`, nil); err != nil {
				s.Fatal("Failed to refresh policies: ", err)
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
