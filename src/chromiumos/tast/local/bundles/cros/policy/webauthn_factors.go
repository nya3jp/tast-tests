// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
	"chromiumos/tast/testing/hwdep"
)

type webauthnTestParam struct {
	fingerprintSupported bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebauthnFactors,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that WebAuthn options are enabled or disabled based on the policy value",
		Contacts: []string{
			"hcyang@google.com", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		Params: []testing.Param{
			{
				Val: webauthnTestParam{fingerprintSupported: false},
			},
			{
				Name:              "fingerprint_test",
				ExtraHardwareDeps: hwdep.D(hwdep.Fingerprint()),
				Val:               webauthnTestParam{fingerprintSupported: true},
			},
		},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.WebAuthnFactors{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.QuickUnlockModeAllowlist{}, pci.VerifiedValue),
		},
	})
}

// WebauthnFactors sets up multiple policies, but only tests behavior that it controls.
// It tests "setup" and "webauthn", but not "quick_unlock" or other auth usages. So it will include
// just enough test cases to verify:
// 1. WebAuthnFactors enabled will enable "setup" the auth method and using it for "webauthn",
// even if all other policies disable it.
//
// 2. WebAuthnFactors disabled will disable using the auth method for "webauthn" even if all other
// policies enabled it, but will not disable "setup" for that auth method.
func WebauthnFactors(ctx context.Context, s *testing.State) {
	// TODO(b/210418148): The external site we currently use keeps record for a registered username for a while,
	// so re-using a username will result in non-identical requests from the server. So for now we need truly
	// random values for username strings so that different test runs don't affect each other.
	rand.Seed(time.Now().UnixNano())

	type testCase struct {
		name            string
		webAuthnFactors policy.WebAuthnFactors
		// Since this policy have similar set of entries and controls whether an auth method can be set with
		// WebAuthnFactors together, we want test cases that verify behaviors are correct when both
		// policies are set and have different values. See comments in testCases.
		quickUnlockModeAllowlist policy.QuickUnlockModeAllowlist
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
			webAuthnFactors:          policy.WebAuthnFactors{Stat: policy.StatusUnset},
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Stat: policy.StatusUnset},
		},
		{
			name:                     "empty",
			webAuthnFactors:          policy.WebAuthnFactors{Val: []string{}},
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Stat: policy.StatusUnset},
		},
		// QuickUnlockModeAllowlist set to empty list shouldn't affect set and webauthn capabilities.
		{
			name:                     "all",
			webAuthnFactors:          policy.WebAuthnFactors{Val: []string{"all"}},
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{}},
		},
		{
			name:                     "pin",
			webAuthnFactors:          policy.WebAuthnFactors{Val: []string{"PIN"}},
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Stat: policy.StatusUnset},
		},
		{
			name:                     "webauthn_empty_quick_unlock_all",
			webAuthnFactors:          policy.WebAuthnFactors{Val: []string{}},
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Val: []string{"all"}},
		},
	}

	fingerprintSupported := s.Param().(webauthnTestParam).fingerprintSupported

	if fingerprintSupported {
		testCases = append(testCases, testCase{
			name:                     "fingerprint",
			webAuthnFactors:          policy.WebAuthnFactors{Val: []string{"FINGERPRINT"}},
			quickUnlockModeAllowlist: policy.QuickUnlockModeAllowlist{Stat: policy.StatusUnset},
		},
		)
	}

	for _, param := range testCases {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

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

			pinCapabilities := getExpectedWebAuthnCapabilities(&param.quickUnlockModeAllowlist, &param.webAuthnFactors, "PIN")

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
				fingerprintCapabilities := getExpectedWebAuthnCapabilities(&param.quickUnlockModeAllowlist, &param.webAuthnFactors, "FINGERPRINT")
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

				if err := verifyInSessionAuthDialog(ctx, cr, tconn, pinCapabilities.webAuthn); err != nil {
					s.Fatal("Failed to verify in session auth dialog: ", err)
				}

				// Delete the PIN so upcoming tests don't get affected.
				if err := ui.LeftClick(nodewith.Name("Password").Role(role.RadioButton))(ctx); err != nil {
					s.Fatal("Failed to delete PIN: ", err)
				}
			}
		})
	}
}

type webAuthnCapabilities struct {
	// Whether the auth method is allowed to be set in OS Settings.
	set bool
	// Whether the auth method is allowed to be used for WebAuthn.
	webAuthn bool
}

func getExpectedWebAuthnCapabilities(quickUnlockModeAllowlist *policy.QuickUnlockModeAllowlist, webauthnFactors *policy.WebAuthnFactors, authMethod string) webAuthnCapabilities {
	set, webAuthn := false, false
	if quickUnlockModeAllowlist.Stat != policy.StatusUnset {
		for _, entry := range quickUnlockModeAllowlist.Val {
			if entry == authMethod || entry == "all" {
				set = true
				// If WebAuthnFactors is unset, the pref value will be inherited from QuickUnlockModeAllowlist.
				if webauthnFactors.Stat == policy.StatusUnset {
					webAuthn = true
				}
			}
		}
	}
	if webauthnFactors.Stat != policy.StatusUnset {
		for _, entry := range webauthnFactors.Val {
			if entry == authMethod || entry == "all" {
				set = true
				webAuthn = true
			}
		}
	}
	return webAuthnCapabilities{
		set,
		webAuthn,
	}
}

func verifyInSessionAuthDialog(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, pinEnabled bool) error {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	conn, err := cr.NewConn(ctx, "https://webauthn.io/")
	if err != nil {
		return errors.Wrap(err, "failed to navigate to test website")
	}
	defer func(ctx context.Context, conn *chrome.Conn) {
		conn.Navigate(ctx, "https://webauthn.io/logout")
		conn.CloseTarget(ctx)
		conn.Close()
	}(cleanupCtx, conn)

	name := randomUsername()
	testing.ContextLogf(ctx, "Username: %s", name)
	// Use a random username because webauthn.io keeps state for each username for a period of time.
	err = conn.Eval(ctx, fmt.Sprintf(`document.getElementById('input-email')._x_model.set("%s")`, name), nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression to set username")
	}

	// Press "Register" button.
	err = conn.Eval(ctx, `document.getElementById('register-button').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression to press register button")
	}

	ui := uiauto.New(tconn)

	// If authenticator type is "Platform", there's only platform option so we don't have to manually click "This device".
	// Choose platform authenticator.
	platformAuthenticatorButton := nodewith.Role(role.Button).Name("This device")
	if err := ui.WithTimeout(2 * time.Second).WaitUntilExists(platformAuthenticatorButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to select platform authenticator from transport selection sheet")
	}
	if err := ui.LeftClick(platformAuthenticatorButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click button for platform authenticator")
	}

	if pinEnabled {
		// Wait for ChromeOS WebAuthn dialog.
		dialog := nodewith.ClassName("AuthDialogWidget")
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
			return errors.Wrap(err, "ChromeOS dialog did not show up")
		}
	}

	return nil
}

// randomUsername generates a random username of length 20.
func randomUsername() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"

	ret := make([]byte, 20)
	for i := range ret {
		ret[i] = letters[rand.Intn(len(letters))]
	}

	return string(ret)
}
