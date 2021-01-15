// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UserNativePrintersAllowed,
		Desc: "Test behavior of UserNativePrintersAllowed policy: check if Add printer button is restricted based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Pre:          pre.User,
	})
}

func UserNativePrintersAllowed(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name           string
		wantRestricted bool                              // wantRestricted is the expected restriction state of the "Add printer" button.
		policy         *policy.UserNativePrintersAllowed // policy is the policy we test.
	}{
		{
			name:           "unset",
			wantRestricted: false,
			policy:         &policy.UserNativePrintersAllowed{Stat: policy.StatusUnset},
		},
		{
			name:           "not_allowed",
			wantRestricted: true,
			policy:         &policy.UserNativePrintersAllowed{Val: false},
		},
		{
			name:           "allowed",
			wantRestricted: false,
			policy:         &policy.UserNativePrintersAllowed{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open settings page where the affected button can be found.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/cupsPrinters")
			if err != nil {
				s.Fatal("Failed to open os settings: ", err)
			}
			defer conn.Close()

			params := ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Add printer",
			}
			// Find the button node.
			node, err := ui.FindWithTimeout(ctx, tconn, params, 15*time.Second)
			if err != nil {
				s.Fatal("Failed to find button node: ", err)
			}
			defer node.Release(ctx)

			// Check the restriction setting of the button.
			if restricted := (node.Restriction == ui.RestrictionDisabled || node.Restriction == ui.RestrictionReadOnly); restricted != param.wantRestricted {
				s.Errorf("Unexpected button restriction in the settings: got %t; want %t", restricted, param.wantRestricted)
			}
		})
	}
}
