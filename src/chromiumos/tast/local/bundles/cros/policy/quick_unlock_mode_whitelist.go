// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: QuickUnlockModeWhitelist,
		Desc: "Checks that the deprecated policy still works",
		Contacts: []string{
			"janagrill@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func QuickUnlockModeWhitelist(ctx context.Context, s *testing.State) {
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

	for _, param := range []struct {
		name                   string
		wantRadioButtonEnabled bool
		policies               []policy.Policy
	}{
		{
			name:                   "unset",
			wantRadioButtonEnabled: false,
			policies: []policy.Policy{
				&policy.QuickUnlockModeWhitelist{Stat: policy.StatusUnset},
			},
		},
		{
			name:                   "empty",
			wantRadioButtonEnabled: false,
			policies: []policy.Policy{
				&policy.QuickUnlockModeWhitelist{Val: []string{}},
			},
		},
		{
			name:                   "all",
			wantRadioButtonEnabled: true,
			policies: []policy.Policy{
				&policy.QuickUnlockModeWhitelist{Val: []string{"all"}},
			},
		},
		{
			name:                   "pin",
			wantRadioButtonEnabled: true,
			policies: []policy.Policy{
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

			// Update policies. Use |ServeAndRefresh| instead of |ServeAndVerify| to
			// avoid the error due to the policy being deprecated.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the Lockscreen page where we can set a PIN.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/lockScreen")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			ui := uiauto.New(tconn)

			// Find and enter the password in the pop up window.
			if err := ui.LeftClick(nodewith.Name("Password").Role(role.TextField))(ctx); err != nil {
				s.Fatal("Could not find the password field: ", err)
			}
			if err := kb.Type(ctx, fixtures.Password+"\n"); err != nil {
				s.Fatal("Failed to type password: ", err)
			}

			// Click on the "PIN or password" radio button.
			if err := ui.LeftClick(nodewith.Name("PIN or password").Role(role.RadioButton))(ctx); err != nil {
				s.Fatal("Failed to click PIN or password radio button: ", err)
			}

			// If the "PIN or password" radio button is disabled, the "Set up PIN"
			// button should not be visible and therefore throw an error when a click
			// is attempted, otherwise a click should succeed.
			if err := ui.LeftClick(nodewith.Name("Set up PIN").Role(role.Button))(ctx); (err == nil) != param.wantRadioButtonEnabled {
				s.Errorf("Unexpected state for PIN or password radio button: got %v, want %v", err == nil, param.wantRadioButtonEnabled)
			}
		})
	}
}
