// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

type vkTestCaseData struct {
	name          string          // name is the subtest name.
	wantedAllowVK bool            // wantedAllowVK describes if virtual keyboard is expected to to shown or not.
	policies      []policy.Policy // policies is the policies values.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: VirtualKeyboard,
		Desc: "Behavior of VirtualKeyboardEnabled and TouchVirtualKeyboardEnabled policies and their mixing by checking that the virtual keyboard (is/is not) displayed as requested by the policy",
		Contacts: []string{
			"kamilszarek@google.com",    // Test author of the merge.
			"giovax@google.com",         // Test author of virtual_keyboard_enabled.go.
			"alexanderhartl@google.com", // Test author of touch_virtual_keyboard_enabled.go.
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Params: []testing.Param{
			{
				Name: "both",
				Val: []vkTestCaseData{
					{
						name:          "vke_enabled-tvke_enabled",
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "vke_enabled-tvke_disabled",
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "vke_disabled-tvke_enabled",
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
				},
			},
			{
				Name: "virtual",
				Val: []vkTestCaseData{
					{
						name:          "enabled",
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "disabled",
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "unset",
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name: "touch_virtual",
				Val: []vkTestCaseData{
					{
						name:          "enabled",
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "disabled",
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "unset",
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
		},
	})
}

// VirtualKeyboard applies VK related policies and uses browser's
// address bar to bring it up. Then asserts according to expectations.
func VirtualKeyboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tcs := s.Param().([]vkTestCaseData)

	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(
				ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+tc.name+".txt")

			// Reset Chrome.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Click the address bar.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "Address and search bar",
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Failed to click address bar: ", err)
			}

			const vkTimeout = 15 * time.Second
			var vkFindParams = ui.FindParams{
				Role: ui.RoleTypeKeyboard,
				Name: "Chrome OS Virtual Keyboard",
			}

			if tc.wantedAllowVK == true {
				// Confirm that the virtual keyboard exists.
				if err := ui.WaitUntilExists(ctx, tconn, vkFindParams, vkTimeout); err != nil {
					s.Errorf("Virtual keyboard did not show up within %s: %s", vkTimeout, err)
				}
			} else {
				// Confirm that the virtual keyboard does not exist.
				if err := policyutil.VerifyNotExists(ctx, tconn, vkFindParams, vkTimeout); err != nil {
					s.Errorf("Virtual keyboard was still visible after %s: %s", vkTimeout, err)
				}
			}
		})
	}
}
