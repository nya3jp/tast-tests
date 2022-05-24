// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast-tests/common/fixture"
	"chromiumos/tast-tests/common/policy"
	"chromiumos/tast-tests/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast-tests/local/chrome"
	"chromiumos/tast-tests/local/chrome/browser"
	"chromiumos/tast-tests/local/chrome/browser/browserfixt"
	"chromiumos/tast-tests/local/chrome/uiauto/checked"
	"chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"chromiumos/tast-tests/local/chrome/uiauto/restriction"
	"chromiumos/tast-tests/local/chrome/uiauto/role"
	"chromiumos/tast-tests/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PasswordManagerEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of PasswordManagerEnabled policy, checking the correspoding toggle button states (restriction and checked) after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

// PasswordManagerEnabled tests the PasswordManagerEnabled policy.
func PasswordManagerEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, param := range []struct {
		name            string
		wantRestriction restriction.Restriction        // wantRestriction is the wanted restriction state of the toggle button for the "Offer to save password" option.
		wantChecked     checked.Checked                // wantChecked is the wanted checked state of the toggle button for the "Offer to save password" option.
		value           *policy.PasswordManagerEnabled // value is the value of the policy.
	}{
		{
			name:            "unset",
			wantRestriction: restriction.None,
			wantChecked:     checked.True,
			value:           &policy.PasswordManagerEnabled{Stat: policy.StatusUnset},
		},
		{
			name:            "forced",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.True,
			value:           &policy.PasswordManagerEnabled{Val: true},
		},
		{
			name:            "deny",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.False,
			value:           &policy.PasswordManagerEnabled{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			if err := policyutil.SettingsPage(ctx, cr, br, "passwords").
				SelectNode(ctx, nodewith.
					Name("Offer to save passwords").
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}
