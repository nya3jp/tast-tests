// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TouchVirtualKeyboardEnabled,
		Desc: "Behavior of TouchVirtualKeyboardEnabled policy, check if a virtual keyboard is opened based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func TouchVirtualKeyboardEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Open a keyboard device.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	for _, param := range []struct {
		name    string
		enabled bool                                // enabled is the expected enabled state of the virtual keyboard.
		policy  *policy.TouchVirtualKeyboardEnabled // policy is the policy we test.
	}{
		{
			name:    "unset",
			enabled: false,
			policy:  &policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset},
		},
		{
			name:    "disabled",
			enabled: false,
			policy:  &policy.TouchVirtualKeyboardEnabled{Val: false},
		},
		{
			name:    "enabled",
			enabled: true,
			policy:  &policy.TouchVirtualKeyboardEnabled{Val: true},
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

			// Try to open a tab.
			if err := keyboard.Accel(ctx, "ctrl+t"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Get the address bar.
			paramsAB := ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "Address and search bar",
			}
			nodeAB, err := ui.FindWithTimeout(ctx, tconn, paramsAB, 30*time.Second)
			if err != nil {
				s.Fatal("Failed to find address bar: ", err)
			}
			defer nodeAB.Release(ctx)

			if err := nodeAB.LeftClick(ctx); err != nil {
				s.Fatal("Failed to click address bar: ", err)
			}

			// Get the virtual keyboard.
			paramsK := ui.FindParams{
				Role: ui.RoleTypeKeyboard,
				Name: "Chrome OS Virtual Keyboard",
			}
			if err := ui.WaitUntilExists(ctx, tconn, paramsK, 30*time.Second); err != nil && !errors.Is(err, ui.ErrNodeDoesNotExist) {
				s.Fatal("Failed to find virtual keyboard: ", err)
			} else if b := !errors.Is(err, ui.ErrNodeDoesNotExist); param.enabled != b {
				s.Errorf("Unexpected existence of virtual keyboard: got %t; want %t", b, param.enabled)
			}
		})
	}
}
