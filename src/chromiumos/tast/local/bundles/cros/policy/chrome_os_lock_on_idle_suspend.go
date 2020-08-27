// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeOsLockOnIdleSuspend,
		Desc: "Behavior of ChromeOsLockOnIdleSuspend policy, checking the correspoding toggle button states (restriction and checked) after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// ChromeOsLockOnIdleSuspend tests the ChromeOsLockOnIdleSuspend policy.
func ChromeOsLockOnIdleSuspend(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Open a keyboard device.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	for _, param := range []struct {
		name           string
		wantRestricted bool                              // wantRestricted is the wanted restriction state of the toggle button for the "Show lock screen when waking from sleep" option.
		wantChecked    ui.CheckedState                   // wantChecked is the wanted checked state of the toggle button for the "Show lock screen when waking from sleep" option.
		value          *policy.ChromeOsLockOnIdleSuspend // value is the value of the policy.
	}{
		{
			name:           "forced",
			wantRestricted: true,
			wantChecked:    ui.CheckedStateTrue,
			value:          &policy.ChromeOsLockOnIdleSuspend{Val: true},
		},
		{
			name:           "disabled",
			wantRestricted: true,
			wantChecked:    ui.CheckedStateFalse,
			value:          &policy.ChromeOsLockOnIdleSuspend{Val: false},
		},
		{
			name:           "unset",
			wantRestricted: false,
			wantChecked:    ui.CheckedStateFalse,
			value:          &policy.ChromeOsLockOnIdleSuspend{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Connect to Test API to use it with the ui library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// Open the Security and sign-in page where the affected toggle button can be found.
			conn, err := cr.NewConn(ctx, "chrome://os-settings/lockScreen")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// The Security and sign-in page is password protected. It asks for the password in a dialog.
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeDialog,
				Name: "Confirm your password",
			}, 15*time.Second); err != nil {
				s.Fatal("Waiting for password dialog failed: ", err)
			}

			// Type the password to unlock the lock screen settings page.
			if err := keyboard.Type(ctx, pre.Password+"\n"); err != nil {
				s.Fatal("Failed to type password: ", err)
			}

			// Find the toggle button node.
			tbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeToggleButton,
				Name: "Show lock screen when waking from sleep",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Finding toggle button failed: ", err)
			}
			defer tbNode.Release(ctx)

			// Check the checked state of the toggle button.
			if tbNode.Checked != param.wantChecked {
				s.Errorf("Unexpected toggle button checked state: got %v; want %v", tbNode.Checked, param.wantChecked)
			}

			// Check the restriction setting of the toggle button.
			if restricted := (tbNode.Restriction == ui.RestrictionDisabled || tbNode.Restriction == ui.RestrictionReadOnly); restricted != param.wantRestricted {
				s.Logf("The restriction state is %q", tbNode.Restriction)
				s.Errorf("Unexpected toggle button restriction: got %t; want %t", restricted, param.wantRestricted)
			}
		})
	}
}
