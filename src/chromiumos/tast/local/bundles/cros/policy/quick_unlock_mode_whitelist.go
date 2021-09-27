// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
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
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromePolicyLoggedIn,
	})
}

func QuickUnlockModeWhitelist(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).ChromeVal()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMSVal()

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
		name            string
		wantRestriction restriction.Restriction
		policies        []policy.Policy
	}{
		{
			name:            "unset",
			wantRestriction: restriction.Disabled,
			policies: []policy.Policy{
				&policy.QuickUnlockModeWhitelist{Stat: policy.StatusUnset},
			},
		},
		{
			name:            "empty",
			wantRestriction: restriction.Disabled,
			policies: []policy.Policy{
				&policy.QuickUnlockModeWhitelist{Val: []string{}},
			},
		},
		{
			name:            "all",
			wantRestriction: restriction.None,
			policies: []policy.Policy{
				&policy.QuickUnlockModeWhitelist{Val: []string{"all"}},
			},
		},
		{
			name:            "pin",
			wantRestriction: restriction.None,
			policies: []policy.Policy{
				&policy.QuickUnlockModeWhitelist{Val: []string{"PIN"}},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

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
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osPrivacy/lockScreen")
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

			// Find node info for the radio button group node.
			rgNode, err := ui.Info(ctx, nodewith.Role(role.RadioGroup))
			if err != nil {
				s.Fatal("Finding radio group failed: ", err)
			}

			// Check that the radio button group has the expected restriction.
			if rgNode.Restriction != param.wantRestriction {
				s.Errorf("Unexpected Continue button state: got %v, want %v", rgNode.Restriction, param.wantRestriction)
			}
		})
	}
}
