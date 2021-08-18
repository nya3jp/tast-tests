// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func SafeBrowsingProtectionLevel(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the security settings page.
			if err := policyutil.SettingsPage(ctx, cr, "security").
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
