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
		Func: SecondaryGoogleAccountSigninAllowed,
		Desc: "Test behavior of SecondaryGoogleAccountSigninAllowed policy: check if Add account button is restricted based on the value of the policy", // TODO(chromium:1128915): Add test cases for signin screen.
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func SecondaryGoogleAccountSigninAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name           string
		wantRestricted bool                                        // wantRestricted is the expected restriction state of the "Add account" button.
		policy         *policy.SecondaryGoogleAccountSigninAllowed // policy is the policy we test.
	}{
		{
			name:           "unset",
			wantRestricted: false,
			policy:         &policy.SecondaryGoogleAccountSigninAllowed{Stat: policy.StatusUnset},
		},
		{
			name:           "not_allowed",
			wantRestricted: true,
			policy:         &policy.SecondaryGoogleAccountSigninAllowed{Val: false},
		},
		{
			name:           "allowed",
			wantRestricted: false,
			policy:         &policy.SecondaryGoogleAccountSigninAllowed{Val: true},
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

			// Open people settings page.
			conn, err := cr.NewConn(ctx, "chrome://settings/people")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Click the Google Account button.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Google Accounts",
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to click Google Accounts button: ", err)
			}

			// We might get a dialog box where we have to click a button before we get to the actual settings we need.
			if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
				s.Fatal("Failed to wait for location change: ", err)
			}
			paramsVA := ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "View accounts",
			}
			if exists, err := ui.Exists(ctx, tconn, paramsVA); err != nil {
				s.Fatal("Unexpected error while checking for View accounts button node: ", err)
			} else if exists {
				if err := ui.StableFindAndClick(ctx, tconn, paramsVA, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
					s.Fatal("Failed to click View accounts button: ", err)
				}
			}

			// Find the Add account button node.
			paramsAA := ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Add account",
			}
			nodeAA, err := ui.FindWithTimeout(ctx, tconn, paramsAA, 15*time.Second)
			if err != nil {
				s.Fatal("Failed to find Add account button node: ", err)
			}
			defer nodeAA.Release(ctx)

			// Check the restriction setting of the Add account button.
			if restricted := (nodeAA.Restriction == ui.RestrictionDisabled || nodeAA.Restriction == ui.RestrictionReadOnly); restricted != param.wantRestricted {
				s.Errorf("Unexpected button restriction in the settings: got %t; want %t", restricted, param.wantRestricted)
			}
		})
	}
}
