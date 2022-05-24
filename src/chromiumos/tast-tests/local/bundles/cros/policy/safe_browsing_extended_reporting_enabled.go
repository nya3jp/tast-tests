// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/errors"
	"chromiumos/tast-tests/local/chrome"
	"chromiumos/tast-tests/local/chrome/browser"
	"chromiumos/tast-tests/local/chrome/browser/browserfixt"
	"chromiumos/tast-tests/local/chrome/uiauto"
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
		Func:         SafeBrowsingExtendedReportingEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of SafeBrowsingExtendedReportingEnabled policy, checking the correspoding toggle button states (restriction and checked) after setting the policy",
		Contacts: []string{
			"laurila@google.com", // Test author
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

func SafeBrowsingExtendedReportingEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name            string
		wantRestriction restriction.Restriction
		wantChecked     checked.Checked
		policy          *policy.SafeBrowsingExtendedReportingEnabled
	}{
		{
			name:            "unset",
			wantRestriction: restriction.None,
			wantChecked:     checked.False,
			policy:          &policy.SafeBrowsingExtendedReportingEnabled{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.True,
			policy:          &policy.SafeBrowsingExtendedReportingEnabled{Val: true},
		},
		{
			name:            "disallow",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.False,
			policy:          &policy.SafeBrowsingExtendedReportingEnabled{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			ui := uiauto.New(tconn)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			settingsPage := policyutil.SettingsPage(ctx, cr, br, "security")
			toggle := nodewith.Name("Help improve security on the web for everyone").Role(role.ToggleButton)

			// Ensure the test works despite the screen size.
			if err := uiauto.Combine("Wait and find the toggle.",
				ui.WaitUntilExists(toggle),
				ui.MakeVisible(toggle),
			)(ctx); err != nil {
				s.Fatal(errors.Wrap(err, "failed to find the toggle"))
			}

			if err := settingsPage.
				SelectNode(ctx, toggle).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}
