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
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinUnlockMaximumLength,
		Desc: "Verify the maximum length of the unlock pin",
		Contacts: []string{
			"janagrill@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func PinUnlockMaximumLength(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	quickUnlockModeAllowlistPolicy := &policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}}

	for _, param := range []struct {
		name     string
		pin      string
		warning  string
		policies []policy.Policy
	}{
		{
			name:    "unset",
			pin:     "13574268135742681357426813574268",
			warning: "",
			policies: []policy.Policy{
				&policy.PinUnlockMaximumLength{Stat: policy.StatusUnset},
				quickUnlockModeAllowlistPolicy,
			},
		},
		{
			name:    "set",
			pin:     "134213421342",
			warning: "PIN must be less than 11 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMaximumLength{Val: 10},
				quickUnlockModeAllowlistPolicy,
			},
		},
		{
			name:    "smaller-than-min",
			pin:     "13574268",
			warning: "PIN must be less than 7 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMaximumLength{Val: 4},
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
			if err := kb.Type(ctx, fixtures.Password+"\n"); err != nil {
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

			// Enter the PIN.
			if err := kb.Type(ctx, param.pin); err != nil {
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

			if len(param.warning) == 0 {
				// If the warning is not expected, just check that the Continue button
				// is enabled.
				if cbNode.Restriction == ui.RestrictionDisabled {
					s.Fatal("Continue button should be enabled")
				}
			} else {
				// If there is a warning message, check that it is displayed and the
				// Continue button is disabled.
				if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
					Role: ui.RoleTypeStaticText,
					Name: param.warning,
				}, 15*time.Second); err != nil {
					s.Fatal("Failed to find the warning message: ", err)
				}

				if cbNode.Restriction != ui.RestrictionDisabled {
					s.Fatal("Continue button should be disabled")
				}
			}
		})
	}
}
