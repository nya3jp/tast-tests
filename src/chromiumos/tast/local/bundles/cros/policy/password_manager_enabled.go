// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PasswordManagerEnabled,
		Desc: "Behavior of PasswordManagerEnabled policy, checking the correspoding toggle button states (restriction and checked) after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// PasswordManagerEnabled tests the PasswordManagerEnabled policy.
func PasswordManagerEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name            string
		wantRestriction ui.RestrictionState            // wantRestriction is the wanted restriction state of the toggle button for the "Offer to save password" option.
		wantChecked     ui.CheckedState                // wantChecked is the wanted checked state of the toggle button for the "Offer to save password" option.
		value           *policy.PasswordManagerEnabled // value is the value of the policy.
	}{
		{
			name:            "unset",
			wantRestriction: ui.RestrictionNone,
			wantChecked:     ui.CheckedStateTrue,
			value:           &policy.PasswordManagerEnabled{Stat: policy.StatusUnset},
		},
		{
			name:            "forced",
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateTrue,
			value:           &policy.PasswordManagerEnabled{Val: true},
		},
		{
			name:            "deny",
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateFalse,
			value:           &policy.PasswordManagerEnabled{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the password settings page where the affected toggle button can be found.
			if err := policyutil.VerifySettingsState(ctx, cr, "chrome://settings/passwords",
				ui.FindParams{
					Role: ui.RoleTypeToggleButton,
					Name: "Offer to save passwords",
				},
				ui.FindParams{
					Attributes: map[string]interface{}{
						"restriction": param.wantRestriction,
						"checked":     param.wantChecked,
					},
				},
			); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}
