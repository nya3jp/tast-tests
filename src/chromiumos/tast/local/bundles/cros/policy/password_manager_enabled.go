// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
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
		Pre:          pre.User,
	})
}

// PasswordManagerEnabled tests the PasswordManagerEnabled policy.
func PasswordManagerEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name        string
		restriction ui.RestrictionState            // restriction is the wanted restriction state of the toggle button for the "Offer to save password" option.
		wantChecked ui.CheckedState                // wantChecked is the wanted checked state of the toggle button for the "Offer to save password" option.
		value       *policy.PasswordManagerEnabled // value is the value of the policy.
	}{
		{
			name:        "unset",
			restriction: ui.RestrictionNone,
			wantChecked: ui.CheckedStateTrue,
			value:       &policy.PasswordManagerEnabled{Stat: policy.StatusUnset},
		},
		{
			name:        "forced",
			restriction: ui.RestrictionDisabled,
			wantChecked: ui.CheckedStateTrue,
			value:       &policy.PasswordManagerEnabled{Val: true},
		},
		{
			name:        "deny",
			restriction: ui.RestrictionDisabled,
			wantChecked: ui.CheckedStateFalse,
			value:       &policy.PasswordManagerEnabled{Val: false},
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
			conn, err := cr.NewConn(ctx, "chrome://settings/passwords")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Find the toggle button node.
			tbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeToggleButton,
				Name: "Offer to save passwords",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Finding toggle button failed: ", err)
			}
			defer tbNode.Release(ctx)

			if isMatched, err := tbNode.MatchesParamsWithEmptyAttributes(ctx, ui.FindParams{
				Attributes: map[string]interface{}{
					"restriction": param.restriction,
					"checked":     param.wantChecked,
				},
			}); err != nil {
				s.Fatal("Failed to check a matching node: ", err)
			} else if isMatched == false {
				s.Errorf("Failed to verify the matching toggle button node; got (%#v, %#v), want (%#v, %#v)", tbNode.Checked, tbNode.Restriction, param.wantChecked, param.restriction)
			}
		})
	}
}
