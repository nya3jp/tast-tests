// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:         PinUnlockMinimumLength,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Follows the user flow to set unlock pin",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"gabormagda@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromePolicyLoggedInLockscreen,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.QuickUnlockModeAllowlist{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.PinUnlockMinimumLength{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func PinUnlockMinimumLength(ctx context.Context, s *testing.State) {
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
				&policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}},
			},
		},
		{
			name:    "shorter",
			pin:     "1342",
			warning: "PIN must be at least 4 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMinimumLength{Val: 4},
				&policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}},
			},
		},
		{
			name:    "longer",
			pin:     "13574268",
			warning: "PIN must be at least 8 digits",
			policies: []policy.Policy{
				&policy.PinUnlockMinimumLength{Val: 8},
				&policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}},
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

			continueButton := nodewith.Name("Continue").Role(role.Button)
			warningMessage := nodewith.Name(param.warning).Role(role.StaticText)

			if err := uiauto.Combine("switch to PIN and wait for PIN dialog",
				// Find and click on radio button "PIN".
				ui.LeftClick(nodewith.Name("PIN").Role(role.RadioButton)),
				// Find and click on "Set up PIN" button.
				ui.LeftClick(nodewith.Name("Set up PIN").Role(role.Button)),
				// Wait for the PIN pop up window to appear.
				ui.WaitUntilExists(warningMessage),
			)(ctx); err != nil {
				s.Fatal("Failed to open PIN dialog with warning message (1): ", err)
			}

			// Entering a good PIN will make Continue button clickable and the warning message will disappear.
			if err := kb.Type(ctx, param.pin); err != nil {
				s.Fatal("Failed to type PIN: ", err)
			}

			// Find the Continue button node.
			if err := ui.WaitUntilExists(continueButton)(ctx); err != nil {
				s.Fatal("Could not find the Continue button: ", err)
			}

			// Wait for the warning message to disappear.
			if err := ui.WaitUntilGone(warningMessage)(ctx); err != nil {
				s.Fatal("The warning message isn't gone yet: ", err)
			}

			// Find the node info for the Continue button.
			nodeInfo, err := ui.Info(ctx, continueButton)
			if err != nil {
				s.Fatal("Could not get info for the Continue button: ", err)
			}

			// Check Continue button state, it must be clickable with a good pin.
			if nodeInfo.Restriction == restriction.Disabled {
				s.Error("The continue button is disabled while the pin is good")
			}

			// Press backspace to remove 1 digit to get a bad pin.
			if err := kb.Accel(ctx, "Backspace"); err != nil {
				s.Fatal("Failed to press backspace: ", err)
			}

			// Wait for the warning message to appear again after removing 1 digit.
			if err := ui.WaitUntilExists(warningMessage)(ctx); err != nil {
				s.Fatal("Failed to find the warning message (2): ", err)
			}

			nodeInfo, err = ui.Info(ctx, continueButton)
			if err != nil {
				s.Fatal("Could not get new info for the Continue button: ", err)
			}
			// Check Continue button state, it must be disabled with a bad pin.
			if nodeInfo.Restriction != restriction.Disabled {
				s.Fatal("Continue button should be disabled")
			}
		})
	}
}
