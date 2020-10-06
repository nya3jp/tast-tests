// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/ossettings"
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
		Attr:         []string{"group:mainline", "informational"},
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
		pinBad   string
		pinGood  string
		policies []policy.Policy
	}{
		{
			name:    "unset",
			pinBad:  "1234",
			pinGood: "123456",
			policies: []policy.Policy{
				&policy.PinUnlockMinimumLength{Stat: policy.StatusUnset},
				&policy.QuickUnlockModeWhitelist{Val: []string{"PIN"}},
			},
		},
		{
			name:    "min_4",
			pinBad:  "12",
			pinGood: "12345",
			policies: []policy.Policy{
				&policy.PinUnlockMinimumLength{Val: 4},
				&policy.QuickUnlockModeWhitelist{Val: []string{"PIN"}},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// launch os-settings at People page
			if err := ossettings.LaunchAtPage(ctx, tconn, ui.FindParams{
				Name: "People",
				Role: ui.RoleTypeLink,
			}); err != nil {
				s.Fatal("Failed to launch Settings: ", err)
			}

			// At People page find and click on Security and sign-in
			nodeSSI, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeLink,
				Name: "Security and sign-in",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Failed to find Security and sign-in option: ", err)
			}
			defer nodeSSI.Release(ctx)
			if err := nodeSSI.StableLeftClick(ctx, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to open the Security and sign-in option: ", err)
			}

			// find and enter the password in the pop up window
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "Password",
			}, 15*time.Second); err != nil {
				s.Fatal("Could not find the password field: ", err)
			}
			if err := kb.Type(ctx, pre.Password+"\n"); err != nil {
				s.Fatal("Failed to type password: ", err)
			}

			// find and click on radio button PIN or password
			if err := ui.FindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeRadioButton,
				Name: "PIN or password",
			}, 15*time.Second); err != nil {
				s.Fatal("Failed to find PIN or password radio button: ", err)
			}

			// find and click on Set up PIN button
			if err := ui.FindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Set up PIN",
			}, 15*time.Second); err != nil {
				s.Fatal("Failed to find Set up PIN button: ", err)
			}

			// wait for the PIN pop up window to appear
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "PIN",
			}, 15*time.Second); err != nil {
				s.Fatal("Could not find the PIN field: ", err)
			}

			// entering a bad PIN won't direct to Confirm the PIN and the Continue button will remain
			if err := kb.Type(ctx, param.pinBad+"\n"); err != nil {
				s.Fatal("Failed to type PIN: ", err)
			}
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Continue",
			}, 15*time.Second); err != nil {
				s.Fatal("Could not find Continue button: ", err)
			}

			// entering a good PIN will make Confirm your PIN appear
			if err := kb.Type(ctx, param.pinGood+"\n"); err != nil {
				s.Fatal("Failed to type PIN: ", err)
			}
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeStaticText,
				Name: "Confirm your PIN",
			}, 15*time.Second); err != nil {
				s.Fatal("Could not find Confirm your PIN dialog: ", err)
			}
		})
	}
}
