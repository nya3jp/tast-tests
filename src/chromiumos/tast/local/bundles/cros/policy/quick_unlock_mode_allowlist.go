// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParam struct {
	fingerprintSupported bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickUnlockModeAllowlist,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that quick unlock options are enabled or disabled based on the policy value",
		Contacts: []string{
			"janagrill@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		Params: []testing.Param{
			{
				Val: testParam{fingerprintSupported: false},
			},
			{
				Name:              "fingerprint_test",
				ExtraHardwareDeps: hwdep.D(hwdep.Fingerprint()),
				Val:               testParam{fingerprintSupported: true},
			},
		},
	})
}

type testCase struct {
	name                     string
	quickUnlockModeAllowlist policy.QuickUnlockModeAllowlist
}

func QuickUnlockModeAllowlist(ctx context.Context, s *testing.State) {
	const PIN = "123456"

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

	testCases := []testCase{
		{
			name:                     "unset",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Stat: policy.StatusUnset},
		},
		{
			name:                     "empty",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{}},
		},
		{
			name:                     "all",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{"all"}},
		},
		{
			name:                     "pin",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}},
		},
	}

	fingerprintSupported := s.Param().(testParam).fingerprintSupported

	if fingerprintSupported {
		testCases = append(testCases, testCase{
			name:                     "fingerprint",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{"FINGERPRINT"}},
		},
		)
	}

	for _, param := range testCases {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			policies := []policy.Policy{
				&param.quickUnlockModeAllowlist,
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
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
				s.Fatal("Failed to find the password field: ", err)
			}
			if err := kb.Type(ctx, fixtures.Password+"\n"); err != nil {
				s.Fatal("Failed to type password: ", err)
			}

			// Find node info for the radio button group node.
			rgNode, err := ui.Info(ctx, nodewith.Role(role.RadioGroup))
			if err != nil {
				s.Fatal("Failed to find radio group: ", err)
			}

			pinCapabilities := getExpectedCapabilities(&param.quickUnlockModeAllowlist, "PIN")

			var wantRestriction restriction.Restriction
			if pinCapabilities.set {
				wantRestriction = restriction.None
			} else {
				wantRestriction = restriction.Disabled
			}

			// Check that the radio button group has the expected restriction.
			if rgNode.Restriction != wantRestriction {
				s.Errorf("Unexpected radio button group state: got %v, want %v", rgNode.Restriction, wantRestriction)
			}

			if fingerprintSupported {
				fingerprintCapabilities := getExpectedCapabilities(&param.quickUnlockModeAllowlist, "FINGERPRINT")
				found, err := ui.IsNodeFound(ctx, nodewith.Name("Edit Fingerprints").Role(role.StaticText))
				if err != nil {
					s.Fatal("Failed to find Edit Fingerprints node: ", err)
				}
				if found != fingerprintCapabilities.set {
					s.Errorf("Failed checking if fingerprint can be set: got %v, want %v", found, fingerprintCapabilities.set)
				}
			}

			if pinCapabilities.unlock {
				if err := uiauto.Combine("switch to PIN or password and wait for PIN dialog",
					// Find and click on radio button "PIN or password".
					ui.LeftClick(nodewith.Name("PIN or password").Role(role.RadioButton)),
					// Find and click on "Set up PIN" button.
					ui.LeftClick(nodewith.Name("Set up PIN").Role(role.Button)),
					// Wait for the PIN pop up window to appear.
					ui.WaitUntilExists(nodewith.Name("Enter your PIN").Role(role.StaticText)),
				)(ctx); err != nil {
					s.Fatal("Failed to open PIN dialog: ", err)
				}

				// Enter the PIN.
				if err := kb.Type(ctx, PIN); err != nil {
					s.Fatal("Failed to type PIN: ", err)
				}

				continueButton := nodewith.Name("Continue").Role(role.Button)

				// Find the Continue button node.
				if err := ui.WaitUntilExists(continueButton)(ctx); err != nil {
					s.Fatal("Failed to find the Continue button: ", err)
				}

				if err := ui.LeftClick(continueButton)(ctx); err != nil {
					s.Fatal("Failed to click the Continue button: ", err)
				}

				if err := ui.WaitUntilExists(nodewith.Name("Confirm your PIN").Role(role.StaticText))(ctx); err != nil {
					s.Fatal("Failed to find the PIN confirmation dialog: ", err)
				}

				// Enter the PIN.
				if err := kb.Type(ctx, PIN); err != nil {
					s.Fatal("Failed to type PIN: ", err)
				}

				confirmButton := nodewith.Name("Confirm").Role(role.Button)

				if err := ui.LeftClick(confirmButton)(ctx); err != nil {
					s.Fatal("Failed to click the Confirm button: ", err)
				}

				// Don't lock the screen before the add PIN operation ended.
				if err := ui.WaitUntilGone(nodewith.Name("Confirm your PIN").Role(role.StaticText))(ctx); err != nil {
					s.Fatal("Failed to wait for PIN confirmation dialog to disappear: ", err)
				}

				if err := lockAndUnlockScreen(ctx, tconn, PIN, false); err != nil {
					s.Fatal("Failed to lock and unlock the screen using PIN: ", err)
				}

				// Delete the PIN so upcoming tests don't get affected.
				if err := ui.LeftClick(nodewith.Name("Password only").Role(role.RadioButton))(ctx); err != nil {
					s.Fatal("Failed to delete PIN: ", err)
				}
			}
		})
	}
}

type capabilities struct {
	set    bool
	unlock bool
}

func getExpectedCapabilities(quickUnlockModeAllowlist *policy.QuickUnlockModeAllowlist, authMethod string) capabilities {
	if quickUnlockModeAllowlist.Stat == policy.StatusUnset {
		return capabilities{
			set:    false,
			unlock: false,
		}
	}
	for _, entry := range quickUnlockModeAllowlist.Val {
		if entry == authMethod || entry == "all" {
			return capabilities{
				set:    true,
				unlock: true,
			}
		}
	}
	return capabilities{
		set:    false,
		unlock: false,
	}
}

func lockAndUnlockScreen(ctx context.Context, tconn *chrome.TestConn, PIN string, autosubmit bool) error {
	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to lock the screen")
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
	}

	// Enter and submit the PIN to unlock the DUT.
	if err := lockscreen.EnterPIN(ctx, tconn, PIN); err != nil {
		return errors.Wrap(err, "failed to enter in PIN")
	}

	if !autosubmit {
		if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to submit PIN")
		}
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "waiting for screen to be unlocked failed (last status %+v)", st)
	}
	return nil
}
