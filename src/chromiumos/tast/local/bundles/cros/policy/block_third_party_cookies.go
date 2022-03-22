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
		Func:         BlockThirdPartyCookies,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test the behavior of BlockThirdPartyCookies policy: check if third party cookies are allowed based on policy value",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
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
	})
}

func BlockThirdPartyCookies(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// radioButtonNames is a list of UI element names in the cookies settings page.
	// The order of the strings should follow the order in the settings page.
	// wantRestriction and wantChecked entries are expected to follow this order as well.
	radioButtonNames := []string{
		"Allow all cookies",
		"Block third-party cookies",
	}

	for _, param := range []struct {
		name            string                    // name is the name of the test case.
		wantRestriction []restriction.Restriction // The expected restriction states of the radio buttons in
		// radioButtonNames.
		wantChecked []checked.Checked // The expected checked states of the radio buttons in
		// radioButtonNames.
		policy *policy.BlockThirdPartyCookies // policy is the policy we test.
	}{
		{
			name:            "unset",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None},
			wantChecked:     []checked.Checked{checked.False, checked.False},
			policy:          &policy.BlockThirdPartyCookies{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.True, checked.False},
			policy:          &policy.BlockThirdPartyCookies{Val: false},
		},
		{
			name: "block",
			// The radio button for "Block third-party cookies" is not disabled in this case as the user can switch
			// between blocking only third party cookies or all cookies for which there is another radio button on
			// the cookies settings page.
			wantRestriction: []restriction.Restriction{restriction.Disabled, restriction.None},
			wantChecked:     []checked.Checked{checked.False, checked.True},
			policy:          &policy.BlockThirdPartyCookies{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Open cookies settings page.
			conn, err := br.NewConn(ctx, "chrome://settings/cookies")
			if err != nil {
				s.Fatal("Failed to open cookies settings: ", err)
			}
			defer conn.Close()

			// Open cookies settings page and check the state of the radio buttons.
			for i, radioButtonName := range radioButtonNames {
				if err := policyutil.CurrentPage(cr).
					SelectNode(ctx, nodewith.
						Role(role.RadioButton).
						Name(radioButtonName)).
					Restriction(param.wantRestriction[i]).
					Checked(param.wantChecked[i]).
					Verify(); err != nil {
					s.Errorf("Unexpected settings state for the %q button: %v", radioButtonName, err)
				}
			}
			// TODO(crbug.com/1186217): Verify that third party cookies are actually blocked.
		})
	}
}
