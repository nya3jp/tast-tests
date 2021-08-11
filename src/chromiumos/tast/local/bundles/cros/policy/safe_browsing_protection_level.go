// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SafeBrowsingProtectionLevel,
		Desc: "Checks if Google Chrome's Safe Browsing feature is enabled and the mode it operates in",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func SafeBrowsingProtectionLevel(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name            string
		wantRestriction ui.RestrictionState // wantRestriction is the wanted restriction state of the checkboxes in Safe Browsing settings page.
		selectedOption  string              // selectedOption is the selected safety level in Safe Browsing settings page.
		value           *policy.SafeBrowsingProtectionLevel
	}{
		{
			name:            "unset",
			wantRestriction: ui.RestrictionNone,
			selectedOption:  "Standard protection",
			value:           &policy.SafeBrowsingProtectionLevel{Stat: policy.StatusUnset},
		},
		{
			name:            "no_protection",
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "No protection (not recommended)",
			value:           &policy.SafeBrowsingProtectionLevel{Val: 0},
		},
		{
			name:            "standard_protection",
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "Standard protection",
			value:           &policy.SafeBrowsingProtectionLevel{Val: 1},
		},
		{
			name:            "enhanced_protection",
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "Enhanced protection",
			value:           &policy.SafeBrowsingProtectionLevel{Val: 2},
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

			// Open the security settings page where the affected radio buttons can be found.
			conn, err := cr.NewConn(ctx, "chrome://settings/security")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Find the radio group node.
			rgNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeRadioGroup}, 15*time.Second)
			if err != nil {
				s.Fatal("Finding radio group failed: ", err)
			}
			defer rgNode.Release(ctx)

			// Find the selected radio button under the radio group.
			srbNode, err := rgNode.FindSelectedRadioButton(ctx)
			if err != nil {
				s.Fatal("Finding the selected radio button failed: ", err)
			}
			defer srbNode.Release(ctx)

			if err := policyutil.CheckNodeAttributes(srbNode, ui.FindParams{
				Attributes: map[string]interface{}{
					"restriction": param.wantRestriction,
					"name":        param.selectedOption,
				},
			}); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}
