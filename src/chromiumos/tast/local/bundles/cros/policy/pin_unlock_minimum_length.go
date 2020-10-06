// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinUnlockMinimumLength,
		Desc: "Follows the user flow to change the wallpaper",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"gabormagda@goggle.com",
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

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	for _, param := range []struct {
		name            string
		wantedPINLength int
		policies        []policy.Policy
	}{
		{
			name:            "unset",
			wantedPINLength: 6,
			policies:        []policy.Policy{&policy.PinUnlockMinimumLength{Stat: policy.StatusUnset}, &policy.QuickUnlockModeWhitelist{Val: []string{"PIN"}}},
		},
		{
			name:            "Pin minimum length 4",
			wantedPINLength: 5,
			policies:        []policy.Policy{&policy.PinUnlockMinimumLength{Val: 4}, &policy.QuickUnlockModeWhitelist{Val: []string{"PIN"}}},
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

			// Context should be shortened here.
			if err := ossettings.LaunchAtPage(ctx, tconn, ui.FindParams{
				Name: "People",
				Role: ui.RoleTypeLink,
			}); err != nil {
				s.Fatal("Failed to launch Settings: ", err)
			}

			nodeSSI, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeLink,
				Name: "Security and sign-in",
			}, 15*time.Second)
			defer nodeSSI.Release(ctx)

			if err := nodeSSI.StableLeftClick(ctx, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Failed to open the Security and sign-in option: ", err)
			}

			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "Password",
			}, 15*time.Second); err != nil {
				s.Fatal("Could not find the password field: ", err)
			}

			kb, err := input.Keyboard(ctx)
			if err != nil {
				s.Fatal("Failed to get keyboard: ", err)
			}
			defer kb.Close()

			if err := kb.Type(ctx, pre.Password+"\n"); err != nil {
				s.Fatal("Failed to type password: ", err)
			}

			if err := ui.FindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeRadioButton,
				Name: "PIN or password",
			}, 15*time.Second); err != nil {
				s.Fatal("Failed to find PIN or password: ", err)
			}

			if err := ui.FindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Set up PIN",
			}, 15*time.Second); err != nil {
				s.Fatal("Failed to find Set up PIN: ", err)
			}

			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "PIN",
			}, 15*time.Second); err != nil {
				s.Fatal("Could not find the PIN field: ", err)
			}

			var pin strings.Builder
			for dig := 1; dig <= param.wantedPINLength; dig++ {
				pin.WriteString(strconv.Itoa(dig))
			}

			if err := kb.Type(ctx, pin.String()+"\n"); err != nil {
				s.Fatal("Failed to type PIN: ", err)
			}

			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeStaticText,
				Name: "Confirm your PIN",
			}, 15*time.Second); err != nil {
				s.Fatal("Could not find the Confirm your PIN dialog: ", err)
			}
		})
	}
}
