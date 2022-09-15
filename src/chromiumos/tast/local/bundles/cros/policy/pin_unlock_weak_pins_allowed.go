// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
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
		Func:         PinUnlockWeakPinsAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify the user cannot set a weak PIN if disallowed by policy",
		Contacts: []string{
			"janagrill@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromePolicyLoggedInLockscreen,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.QuickUnlockModeAllowlist{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.PinUnlockWeakPinsAllowed{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func PinUnlockWeakPinsAllowed(ctx context.Context, s *testing.State) {
	const (
		// Weak PIN, as defined in IsPinDifficultEnough (quick_unlock_private_api.cc).
		pin       = "123456"
		pinHidden = "••••••"
	)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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
		name              string
		buttonRestriction restriction.Restriction
		policies          []policy.Policy
	}{
		{
			name:              "unset",
			buttonRestriction: restriction.None,
			policies: []policy.Policy{
				&policy.PinUnlockWeakPinsAllowed{Stat: policy.StatusUnset},
				quickUnlockModeAllowlistPolicy,
			},
		},
		{
			name:              "allowed",
			buttonRestriction: restriction.None,
			policies: []policy.Policy{
				&policy.PinUnlockWeakPinsAllowed{Val: true},
				quickUnlockModeAllowlistPolicy,
			},
		},
		{
			name:              "disallowed",
			buttonRestriction: restriction.Disabled,
			policies: []policy.Policy{
				&policy.PinUnlockWeakPinsAllowed{Val: false},
				quickUnlockModeAllowlistPolicy,
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
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

			if err := uiauto.Combine("switch to PIN and wait for PIN dialog",
				// Find and click on radio button "PIN".
				ui.LeftClick(nodewith.Name("PIN").Role(role.RadioButton)),
				// Find and click on "Set up PIN" button.
				ui.LeftClick(nodewith.Name("Set up PIN").Role(role.Button)),
				// Wait for the PIN pop up window to appear.
				ui.WaitUntilExists(nodewith.Name("Enter your PIN").Role(role.StaticText)),
			)(ctx); err != nil {
				s.Fatal("Failed to open PIN dialog: ", err)
			}

			// Enter the PIN, which is very easy to guess. The warning message "PIN
			// may be easy to guess" will appear in any case, but if weak passwords
			// are forbidden, the Continue button will stay disabled.
			if err := uiauto.Combine("enter PIN",
				kb.TypeAction(pin),
				// Wait until all 6 digits are in, and UI is refreshed.
				ui.WaitUntilExists(nodewith.Name(pinHidden).Role(role.InlineTextBox)),
			)(ctx); err != nil {
				s.Fatal("Failed to enter PIN: ", err)
			}

			// Sometimes the update on the Continue button is slightly delayed.
			// Polling here would not work for the restricted state, because that is the state we start from.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			// Find the node info for the Continue button.
			nodeInfo, err := ui.Info(ctx, nodewith.Name("Continue").Role(role.Button))
			if err != nil {
				s.Fatal("Could not get info for the Continue button: ", err)
			}

			if nodeInfo.Restriction != param.buttonRestriction {
				s.Fatalf("Unexpected Continue button state: got %v, want %v", nodeInfo.Restriction, param.buttonRestriction)
			}
		})
	}
}
