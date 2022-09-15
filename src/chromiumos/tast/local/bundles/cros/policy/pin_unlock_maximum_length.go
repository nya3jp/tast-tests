// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

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
		Func:         PinUnlockMaximumLength,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify the maximum length of the unlock PIN",
		Contacts: []string{
			"janagrill@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromePolicyLoggedInLockscreen,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.QuickUnlockModeAllowlist{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.PinUnlockMaximumLength{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func PinUnlockMaximumLength(ctx context.Context, s *testing.State) {
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
			pin:     "13421342134",
			warning: "PIN must be less than 11 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMaximumLength{Val: 10},
				quickUnlockModeAllowlistPolicy,
			},
		},
		{
			name:    "smaller-than-min",
			pin:     "1357426",
			warning: "PIN must be less than 7 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMaximumLength{Val: 4},
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

			// Enter the PIN.
			if err := kb.Type(ctx, param.pin); err != nil {
				s.Fatal("Failed to type PIN: ", err)
			}

			continueButton := nodewith.Name("Continue").Role(role.Button)

			// Find the Continue button node.
			if err := ui.WaitUntilExists(continueButton)(ctx); err != nil {
				s.Fatal("Could not find the Continue button: ", err)
			}

			// Find the node info for the Continue button
			nodeInfo, err := ui.Info(ctx, continueButton)
			if err != nil {
				s.Fatal("Could not get info for the Continue button: ", err)
			}

			if len(param.warning) == 0 {
				// If no warning is expected, check that the Continue button is enabled.
				if nodeInfo.Restriction == restriction.Disabled {
					s.Error("Continue button should be enabled")
				}
			} else {
				// If there is a warning message, check that it is displayed.
				if err := ui.WaitUntilExists(nodewith.Name(param.warning).Role(role.StaticText))(ctx); err != nil {
					s.Fatal("Failed to find the warning message: ", err)
				}

				// Also check that Continue button is disabled.
				if nodeInfo.Restriction != restriction.Disabled {
					s.Fatal("Continue button should be disabled")
				}

				// Press backspace to remove 1 digit to get a good PIN.
				if err := kb.Accel(ctx, "Backspace"); err != nil {
					s.Fatal("Failed to press backspace: ", err)
				}

				// PIN is good again so Continue button should be enabled.
				nodeInfo, err = ui.Info(ctx, continueButton)
				if err != nil {
					s.Fatal("Could not get new info for the Continue button: ", err)
				}

				if nodeInfo.Restriction == restriction.Disabled {
					s.Error("Continue button should be enabled again")
				}
			}
		})
	}
}
