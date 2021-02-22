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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

type testCaseData struct {
	name     string          // name is the subtest name.
	enabled  bool            // enabled is the expected value of the policy once set.
	policies []policy.Policy // policies is the policies values.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: VirtualKeyboardPolicies,
		Desc: "Behavior of VirtualKeyboardEnabled and TouchVirtualKeyboardEnabled policies and their mixing by checking that the virtual keyboard (is/is not) displayed as requested by the policy",
		Contacts: []string{
			"giovax@google.com",         // Test author.
			"alexanderhartl@google.com", // Test author of merged into this touch_virtual_keyboard_enabled.go.
			"kamilszarek@google.com",    // Test author of doing the merge.
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Params: []testing.Param{
			{
				Name: "mixed_touch_virtual_keyboard_with_virtual_keyboard",
				Val: []testCaseData{
					{
						name:     "vke_enabled-tvke_enabled",
						enabled:  true,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:     "vke_enabled-tvke_disabled",
						enabled:  true,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:     "vke_disabled-tvke_enabled",
						enabled:  true,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
				},
			},
			{
				Name: "virtual_keyboard_enabled",
				Val: []testCaseData{
					{
						name:     "enabled",
						enabled:  true,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}},
					},
					{
						name:     "disabled",
						enabled:  false,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}},
					},
					{
						name:     "unset",
						enabled:  false,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name: "touch_virtual_keyboard_enabled",
				Val: []testCaseData{
					{
						name:     "enabled",
						enabled:  true,
						policies: []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:     "disabled",
						enabled:  false,
						policies: []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:     "unset",
						enabled:  false,
						policies: []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
		},
	})
}

// VirtualKeyboardPolicies applies VK related policies and uses browser's
// address bar to bring it up. Then asserts according to expectations.
func VirtualKeyboardPolicies(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Open a keyboard device.
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	tcs, _ := s.Param().([]testCaseData)

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

			// Open a tab.
			if err := keyboard.Accel(ctx, "Ctrl+t"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Click the address bar.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "Address and search bar",
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Failed to click address bar: ", err)
			}

			// TODO: Change the function to NOT ambiguous -> Remove the Bool from the parameters.
			// Confirm the status of the  virtual keyboard node.
			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeKeyboard,
				Name: "Chrome OS Virtual Keyboard",
			}, tc.enabled, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the virtual keyboard: ", err)
			}
		})
	}
}
