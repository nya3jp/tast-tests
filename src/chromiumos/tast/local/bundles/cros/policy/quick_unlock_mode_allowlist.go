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
		Fixture:      fixture.ChromePolicyLoggedInLockscreen,
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
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.QuickUnlockModeAllowlist{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.WebAuthnFactors{}, pci.VerifiedValue),
		},
	})
}

// QuickUnlockModeAllowlist sets up multiple policies, but only tests behavior that it controls.
// It tests "setup" and "quick_unlock", but not "webauthn" or other auth usages. So it will include
// just enough test cases to verify:
// 1. QuickUnlockModeAllowlist enabled will enable "setup" the auth method and using it for "quick_unlock",
//
//	even if all other policies disable it.
//
// 2. QuickUnlockModeAllowlist disabled will disable using the auth method for "quick_unlock" even if all other
//
//	policies enabled it, but will not disable "setup" for that auth method.
func QuickUnlockModeAllowlist(ctx context.Context, s *testing.State) {
	type testCase struct {
		name                     string
		quickUnlockModeAllowlist policy.QuickUnlockModeAllowlist
		// Since this policy have similar set of entries and controls whether an auth method can be set with
		// QuickUnlockModeAllowlist together, we want test cases that verify behaviors are correct when both
		// policies are set and have different values. See comments in quickUnlockTestCases.
		webAuthnFactors policy.WebAuthnFactors
	}

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
			webAuthnFactors:          policy.WebAuthnFactors{Stat: policy.StatusUnset},
		},
		{
			name:                     "empty",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{}},
			webAuthnFactors:          policy.WebAuthnFactors{Stat: policy.StatusUnset},
		},
		// WebAuthnFactors set to empty list shouldn't affect set and unlock capabilities.
		{
			name:                     "all",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{"all"}},
			webAuthnFactors:          policy.WebAuthnFactors{Val: []string{}},
		},
		{
			name:                     "pin",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}},
			webAuthnFactors:          policy.WebAuthnFactors{Stat: policy.StatusUnset},
		},
		// WebAuthnFactors set to all will allow user to set up PIN, but shouldn't allow user
		// to unlock screen using PIN.
		{
			name:                     "quick_unlock_empty_webauthn_all",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{}},
			webAuthnFactors:          policy.WebAuthnFactors{Val: []string{"all"}},
		},
	}

	fingerprintSupported := s.Param().(testParam).fingerprintSupported

	if fingerprintSupported {
		testCases = append(testCases, testCase{
			name:                     "fingerprint",
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{"FINGERPRINT"}},
			webAuthnFactors:          policy.WebAuthnFactors{Stat: policy.StatusUnset},
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
				&param.webAuthnFactors,
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

			pinCapabilities := getExpectedQuickUnlockCapabilities(&param.quickUnlockModeAllowlist, &param.webAuthnFactors, "PIN")

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
				fingerprintCapabilities := getExpectedQuickUnlockCapabilities(&param.quickUnlockModeAllowlist, &param.webAuthnFactors, "FINGERPRINT")
				found, err := ui.IsNodeFound(ctx, nodewith.Name("Edit Fingerprints").Role(role.StaticText))
				if err != nil {
					s.Fatal("Failed to find Edit Fingerprints node: ", err)
				}
				if found != fingerprintCapabilities.set {
					s.Errorf("Failed checking if fingerprint can be set: got %v, want %v", found, fingerprintCapabilities.set)
				}
			}

			// If PIN can be set, we set up a PIN and see if the lock screen UI corresponds to PIN's unlock capability.
			if pinCapabilities.set {
				if err := uiauto.Combine("switch to PINand wait for PIN dialog",
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

				if err := lockAndUnlockScreen(ctx, tconn, kb, fixtures.Password, PIN, pinCapabilities.unlock); err != nil {
					s.Fatal("Failed to lock and unlock the screen using PIN: ", err)
				}

				// Delete the PIN so upcoming tests don't get affected.
				if err := ui.LeftClick(nodewith.Name("Password").Role(role.RadioButton))(ctx); err != nil {
					s.Fatal("Failed to delete PIN: ", err)
				}
			}
		})
	}
}

type quickUnlockCapabilities struct {
	// Whether the auth method is allowed to be set in OS Settings.
	set bool
	// Whether the auth method is allowed to be used for unlocking the screen.
	unlock bool
}

func getExpectedQuickUnlockCapabilities(quickUnlockModeAllowlist *policy.QuickUnlockModeAllowlist, webauthnFactors *policy.WebAuthnFactors, authMethod string) quickUnlockCapabilities {
	set, unlock := false, false
	if quickUnlockModeAllowlist.Stat != policy.StatusUnset {
		for _, entry := range quickUnlockModeAllowlist.Val {
			if entry == authMethod || entry == "all" {
				set = true
				unlock = true
			}
		}
	}
	if webauthnFactors.Stat != policy.StatusUnset {
		for _, entry := range webauthnFactors.Val {
			if entry == authMethod || entry == "all" {
				set = true
			}
		}
	}
	return quickUnlockCapabilities{
		set,
		unlock,
	}
}

func lockAndUnlockScreen(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, password, PIN string, pinEnabled bool) error {
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to lock the screen")
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
	}

	if pinEnabled {
		if err := lockscreen.EnterPIN(ctx, tconn, kb, PIN); err != nil {
			return errors.Wrap(err, "failed to enter in PIN")
		}

		if err := lockscreen.SubmitPINOrPassword(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to submit PIN")
		}
	} else {
		if err := kb.Type(ctx, password+"\n"); err != nil {
			return errors.Wrap(err, "failed to enter password")
		}
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "waiting for screen to be unlocked failed (last status %+v)", st)
	}
	return nil
}
