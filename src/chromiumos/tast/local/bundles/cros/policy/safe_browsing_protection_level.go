// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SafeBrowsingProtectionLevel,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Google Chrome's Safe Browsing feature is enabled and the mode it operates in",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.SafeBrowsingProtectionLevel{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func SafeBrowsingProtectionLevel(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, param := range []struct {
		name            string
		wantRestriction restriction.Restriction // wantRestriction is the wanted restriction state of the radio buttons in Safe Browsing settings page.
		selectedOption  string                  // selectedOption is the selected safety level in Safe Browsing settings page.
		value           *policy.SafeBrowsingProtectionLevel
	}{
		{
			name:            "unset",
			wantRestriction: restriction.None,
			selectedOption:  "Standard protection",
			value:           &policy.SafeBrowsingProtectionLevel{Stat: policy.StatusUnset},
		},
		{
			name:            "no_protection",
			wantRestriction: restriction.Disabled,
			selectedOption:  "No protection (not recommended)",
			value:           &policy.SafeBrowsingProtectionLevel{Val: 0},
		},
		{
			name:            "standard_protection",
			wantRestriction: restriction.Disabled,
			selectedOption:  "Standard protection",
			value:           &policy.SafeBrowsingProtectionLevel{Val: 1},
		},
		{
			name:            "enhanced_protection",
			wantRestriction: restriction.Disabled,
			selectedOption:  "Enhanced protection",
			value:           &policy.SafeBrowsingProtectionLevel{Val: 2},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
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

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open the security settings page.
			if err := policyutil.SettingsPage(ctx, cr, br, "security").
				SelectNode(ctx, nodewith.
					Role(role.RadioButton).
					Name(param.selectedOption)).
				Checked(checked.True).
				Restriction(param.wantRestriction).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}
		})
	}
}
