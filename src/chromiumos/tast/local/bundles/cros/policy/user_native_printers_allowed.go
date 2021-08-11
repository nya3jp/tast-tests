// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
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
		Func: UserNativePrintersAllowed,
		Desc: "Test behavior of UserNativePrintersAllowed policy: check if Add printer button is restricted based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		Fixture: "chromePolicyLoggedIn",
	})
}

func UserNativePrintersAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, param := range []struct {
		name            string
		wantRestriction restriction.Restriction           // wantRestricted is the expected restriction state of the "Add printer" button.
		policy          *policy.UserNativePrintersAllowed // policy is the policy we test.
	}{
		{
			name:            "unset",
			wantRestriction: restriction.None,
			policy:          &policy.UserNativePrintersAllowed{Stat: policy.StatusUnset},
		},
		{
			name:            "not_allowed",
			wantRestriction: restriction.Disabled,
			policy:          &policy.UserNativePrintersAllowed{Val: false},
		},
		{
			name:            "allowed",
			wantRestriction: restriction.None,
			policy:          &policy.UserNativePrintersAllowed{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Check if the Add printer button is restricted.
			if err := policyutil.OSSettingsPage(ctx, cr, "cupsPrinters").
				SelectNode(ctx, nodewith.
					Name("Add printer").
					Role(role.Button)).
				Restriction(param.wantRestriction).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}
		})
	}
}
