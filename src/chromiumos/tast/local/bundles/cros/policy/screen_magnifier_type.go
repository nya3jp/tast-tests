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
		Func: ScreenMagnifierType,
		Desc: "Behavior of ScreenMagnifierType policy",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func ScreenMagnifierType(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name                   string                      // name is the subtest name.
		screenMagnifierEnabled bool                        // screenMagnifierEnabled is the expected enabled state of the screen magnifier.
		dockedMagnifierEnabled bool                        // dockedMagnifierEnabled is the expected enabled state of the docked magnifier.
		value                  *policy.ScreenMagnifierType // value is the policy value.
	}{
		{
			name:                   "disabled",
			screenMagnifierEnabled: false,
			dockedMagnifierEnabled: false,
			value:                  &policy.ScreenMagnifierType{Val: 0},
		},
		{
			name:                   "enabled",
			screenMagnifierEnabled: true,
			dockedMagnifierEnabled: false,
			value:                  &policy.ScreenMagnifierType{Val: 1}, // 1 = Full-screen magnifier enabled
		},
		{
			name:                   "docked magnifier enabled",
			screenMagnifierEnabled: false,
			dockedMagnifierEnabled: true,
			value:                  &policy.ScreenMagnifierType{Val: 2}, // 2 = Docked magnifier enabled
		},
		{
			name:                   "unset",
			screenMagnifierEnabled: false,
			dockedMagnifierEnabled: false,
			value:                  &policy.ScreenMagnifierType{Stat: policy.StatusUnset},
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
			// Check the actual enabled state in accessibilityFeatures.
			for _, item := range []struct {
				name    string // name is the name of the feature in accessibilityFeatures
				enabled bool   // enabled is the expected enabled state of the feature
			}{
				{
					name:    "screenMagnifier",
					enabled: param.screenMagnifierEnabled,
				},
				{
					name:    "dockedMagnifier",
					enabled: param.dockedMagnifierEnabled,
				},
			} {
				var value bool
				if err := tconn.Call(ctx, &value, `async (name) => {
					let result = await tast.promisify(tast.bind(chrome.accessibilityFeatures[name], "get"))({});
					return result.value;
				  }`, item.name); err != nil {
					s.Fatalf("Failed to retrieve %s enabled value: %q", item.name, err)
				}
				if item.enabled != value {
					s.Errorf("Unexpected value of chrome.accessibilityFeatures.%s: got %t; want %t", item.name, value, item.enabled)
				}
			}
		})
	}
}
