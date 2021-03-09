// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinUnlockWeakPinsAllowed,
		Desc: "Verify the user cannot set a weak PIN if disallowed by policy",
		Contacts: []string{
			"janagrill@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.User,
	})
}

func PinUnlockWeakPinsAllowed(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Weak PIN, as defined in IsPinDifficultEnough (quick_unlock_private_api.cc)
	pin := "123456"
	quickUnlockModeAllowlistPolicy := &policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}}

	for _, param := range []struct {
		name                    string
		shouldContinueBeEnabled bool
		policies                []policy.Policy
	}{
		{
			name:                    "unset",
			shouldContinueBeEnabled: true,
			policies: []policy.Policy{
				&policy.PinUnlockWeakPinsAllowed{Stat: policy.StatusUnset},
				quickUnlockModeAllowlistPolicy,
			},
		},
		{
			name:                    "allowed",
			shouldContinueBeEnabled: true,
			policies: []policy.Policy{
				&policy.PinUnlockWeakPinsAllowed{Val: true},
				quickUnlockModeAllowlistPolicy,
			},
		},
		{
			name:                    "disallowed",
			shouldContinueBeEnabled: false,
			policies: []policy.Policy{
				&policy.PinUnlockWeakPinsAllowed{Val: false},
				quickUnlockModeAllowlistPolicy,
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the Lockscreen page where we can set a PIN.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/lockScreen")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Find and enter the password in the pop up window.
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: "Password"}, 15*time.Second); err != nil {
				s.Fatal("Could not find the password field: ", err)
			}
			if err := kb.Type(ctx, pre.Password+"\n"); err != nil {
				s.Fatal("Failed to type password: ", err)
			}

			// Find and click on radio button PIN or password.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeRadioButton,
				Name: "PIN or password",
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to find PIN or password radio button: ", err)
			}

			// Find and click on Set up PIN button.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Set up PIN",
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to find Set up PIN button: ", err)
			}

			// Wait for the PIN pop up window to appear.
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeStaticText,
				Name: "Enter your PIN",
			}, 15*time.Second); err != nil {
				s.Fatal("Failed to find the PIN popup: ", err)
			}

			// Enter the PIN, which is very easy to guess. The warning message "PIN
			// may be easy to guess" will appear in any case, but if weak passwords
			// are forbidden, the Continue button will stay disabled.
			if err := kb.Type(ctx, pin); err != nil {
				s.Fatal("Failed to type PIN: ", err)
			}

			// Find the Continue button node.
			cbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Continue",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Could not find Continue button: ", err)
			}
			defer cbNode.Release(ctx)

			// Check that Continue button is in the correct state.
			isContinueEnabled := cbNode.Restriction != ui.RestrictionDisabled
			if isContinueEnabled != param.shouldContinueBeEnabled {
				s.Fatalf("Unexpected Continue button state: got %v, want %v", isContinueEnabled, param.shouldContinueBeEnabled)
			}
		})
	}
}
