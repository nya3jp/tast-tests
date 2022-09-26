// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowLogoutButtonInTray,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of ShowLogoutButtonInTray policy, check if a logout button is shown based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ShowLogoutButtonInTray{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func ShowLogoutButtonInTray(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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
