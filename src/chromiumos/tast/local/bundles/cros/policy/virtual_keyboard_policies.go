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
	wanted   bool            // wanted is the expected value of the policy once set.
	policies []policy.Policy // policies is the policies values.
}

// vkTimeout used for checking existence or lack of it of the virtual keyboard.
const vkTimeout = 15 * time.Second

// exist constant introduced for readability in the test assertion.
const exist = true

// notExist constant introduced for readability in the test assertion.
const notExist = false

// vkFindParams find parameters for the virtual keyboard.
var vkFindParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeKeyboard,
	Name: "Chrome OS Virtual Keyboard",
}

func init() {
	testing.AddTest(&testing.Test{
		Func: VirtualKeyboardPolicies,
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
				Val: []testCaseData{
					{
						name:     "vke_enabled-tvke_enabled",
						wanted:   true,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:     "vke_enabled-tvke_disabled",
						wanted:   true,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:     "vke_disabled-tvke_enabled",
						wanted:   true,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
				},
			},
			{
				Name: "virtual",
				Val: []testCaseData{
					{
						name:     "enabled",
						wanted:   true,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}},
					},
					{
						name:     "disabled",
						wanted:   false,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}},
					},
					{
						name:     "unset",
						wanted:   false,
						policies: []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name: "touch_virtual",
				Val: []testCaseData{
					{
						name:     "enabled",
						wanted:   true,
						policies: []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:     "disabled",
						wanted:   false,
						policies: []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:     "unset",
						wanted:   false,
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

			// Confirm that the virtual keyboard exist
			if tc.wanted == exist {
				if err := ui.WaitUntilExists(ctx, tconn, vkFindParams, vkTimeout); err != nil {
					s.Errorf("Virtual keyboard did not show up within %s: ", err)
				}
			}

			// Confirm that the virtual keyboard does not exist
			if tc.wanted == notExist {
				if err := policyutil.VerifyNotExists(ctx, tconn, vkFindParams, vkTimeout); err != nil {
					s.Errorf("Virtual keyboard was still visible after %s: ", err)
				}
			}
		})
	}
}
