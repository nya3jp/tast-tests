// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VirtualKeyboardEnabled,
		Desc: "Behavior of VirtualKeyboardEnabled policy by verifying that the desired policy value is actually stored, and by checking that the virtual keyboard (is/is not) displayed as requested by the policy",
		Contacts: []string{
			"giovax@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func VirtualKeyboardEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

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

	for _, param := range []struct {
		name     string                         // name is the subtest name.
		expected bool                           // expected enabled state of the virtual keyboard.
		value    *policy.VirtualKeyboardEnabled // value is the policy value.
	}{
		{
			name:     "true",
			expected: true,
			value:    &policy.VirtualKeyboardEnabled{Val: true},
		},
		{
			name:     "false",
			expected: false,
			value:    &policy.VirtualKeyboardEnabled{Val: false},
		},
		{
			name:     "unset",
			expected: false,
			value:    &policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(
				ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Try to open a tab.
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

			// If the keyboard is not supposed to appear, wait before checking if it appeared.
			if !param.expected {
				if err := testing.Sleep(ctx, 15*time.Second); err != nil {
					s.Fatal("Failed while waiting for the keyboard: ", err)
				}
			}

			// Confirm the status of the  virtual keyboard node.
			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeKeyboard,
				Name: "Chrome OS Virtual Keyboard",
			}, param.expected, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the virtual keyboard: ", err)
			}
		})
	}
}
