// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShowLogoutButtonInTray,
		Desc: "Behavior of ShowLogoutButtonInTray policy, check if a logout button is shown based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func ShowLogoutButtonInTray(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name       string
		wantButton bool                           // wantButton is the expected existence of the "Sign out" button.
		policy     *policy.ShowLogoutButtonInTray // policy is the policy we test.
	}{
		{
			name:       "unset",
			wantButton: false,
			policy:     &policy.ShowLogoutButtonInTray{Stat: policy.StatusUnset},
		},
		{
			name:       "don't show",
			wantButton: false,
			policy:     &policy.ShowLogoutButtonInTray{Val: false},
		},
		{
			name:       "show",
			wantButton: true,
			policy:     &policy.ShowLogoutButtonInTray{Val: true},
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

			// Confirm the status of the Sign out button node.
			ui := uiauto.New(tconn)
			signOutButton := nodewith.Name("Sign out").Role(role.Button).First()
			if err = ui.WaitUntilExists(signOutButton)(ctx); err != nil {
				if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
					s.Fatal("Failed to wait for 'Sign out' button: ", err)
				}
				if param.wantButton {
					s.Error("'Sign out' button not found: ", err)
				}
			} else if !param.wantButton {
				s.Error("Unexpected 'Sign out' button found: ", err)
			}
		})
	}
}
