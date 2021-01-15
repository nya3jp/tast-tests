// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinUnlockMinimumLength,
		Desc: "Follows the user flow to set unlock pin",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"gabormagda@google.com",
			"chromeos-commercial-stability@google.com",
		},
		// TODO(crbug.com/1149286) Disable the test until it can be fixed
		// Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.User,
	})
}

func PinUnlockMinimumLength(ctx context.Context, s *testing.State) {
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

	for _, param := range []struct {
		name     string
		pin      string
		warning  string
		policies []policy.Policy
	}{
		{
			name:    "unset",
			pin:     "135246",
			warning: "PIN must be at least 6 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMinimumLength{Stat: policy.StatusUnset},
				&policy.QuickUnlockModeWhitelist{Val: []string{"PIN"}},
			},
		},
		{
			name:    "shorter",
			pin:     "1342",
			warning: "PIN must be at least 4 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMinimumLength{Val: 4},
				&policy.QuickUnlockModeWhitelist{Val: []string{"PIN"}},
			},
		},
		{
			name:    "longer",
			pin:     "13574268",
			warning: "PIN must be at least 8 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMinimumLength{Val: 8},
				&policy.QuickUnlockModeWhitelist{Val: []string{"PIN"}},
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

			// Wait for the PIN pop up window to appear and the warning message to appear.
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeStaticText,
				Name: param.warning,
			}, 15*time.Second); err != nil {
				s.Fatal("Failed to find the warning message (1): ", err)
			}

			// Entering a good PIN will make Continue button clickable and the warning message will disappear.
			if err := kb.Type(ctx, param.pin); err != nil {
				s.Fatal("Failed to type PIN: ", err)
			}

			// Wait for the warning message to disappear.
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeStaticText,
				Name: param.warning,
			}, 15*time.Second); err != nil {
				s.Fatal("The warning message isn't gone yet: ", err)
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

			// Check Continue button state, it must be clickable with a good pin.
			if cbNode.Restriction == ui.RestrictionDisabled {
				s.Fatal("The continue button is disabled while the pin is good")
			}

			// Press backspace to remove 1 digit to get a bad pin.
			if err := kb.Accel(ctx, "Backspace"); err != nil {
				s.Fatal("Failed to press backspace: ", err)
			}

			// Wait for the warning message to appear again after removing 1 digit.
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeStaticText,
				Name: param.warning,
			}, 15*time.Second); err != nil {
				s.Fatal("Failed to find the warning message (2): ", err)
			}

			// Check Continue button state, it must be disabled with a bad pin.
			cbNode.Update(ctx)
			if cbNode.Restriction != ui.RestrictionDisabled {
				s.Fatal("The continue button is allowed while the pin is bad")
			}
		})
	}
}
