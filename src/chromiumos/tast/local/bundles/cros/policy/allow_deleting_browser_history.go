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
		Func: AllowDeletingBrowserHistory,
		Desc: "Behavior of AllowDeletingBrowserHistory policy, checking the correspoding checkbox states (restriction and checked) after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// AllowDeletingBrowserHistory tests the AllowDeletingBrowserHistory policy.
func AllowDeletingBrowserHistory(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name            string
		wantRestriction ui.RestrictionState                 // wantRestriction is the wanted restriction state of the checkboxes in Browsing history.
		wantChecked     ui.CheckedState                     // wantChecked is the wanted checked state of the checkboxes in Browsing history.
		value           *policy.AllowDeletingBrowserHistory // value is the value of the policy.
	}{
		{
			name:            "unset",
			wantRestriction: ui.RestrictionNone,
			wantChecked:     ui.CheckedStateTrue,
			value:           &policy.AllowDeletingBrowserHistory{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantRestriction: ui.RestrictionNone,
			wantChecked:     ui.CheckedStateTrue,
			value:           &policy.AllowDeletingBrowserHistory{Val: true},
		},
		{
			name:            "deny",
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateFalse,
			value:           &policy.AllowDeletingBrowserHistory{Val: false},
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

			// Open settings page where the affected checkboxes can be found.
			conn, err := cr.NewConn(ctx, "chrome://settings/clearBrowserData")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Loop for different checkboxes.
			// TODO(https://crbug.com/1158773): Use regex in the node names in the AllowDeletingBrowserHistory policy test.
			for _, cb := range []struct {
				ref  string // ref is the shortened name of the checkbox that can be used in logging.
				name string // name is a unique part of the checkbox name in the UI tree.
				tab  string // tab is the name of the tab in the UI tree that should be selected to find the checkbox.
			}{
				{
					ref:  "Browsing history",
					name: "Browsing history Clears history and autocompletions in the search box",
					tab:  "Basic",
				},
				{
					ref:  "Browsing history",
					name: "Browsing history None",
					tab:  "Advanced",
				},
				{
					ref:  "Download history",
					name: "Download history None",
					tab:  "Advanced",
				},
			} {
				// Select the tab if it is not selected already.
				tabNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
					Role: ui.RoleTypeTab,
					Name: cb.tab,
				}, 15*time.Second)
				if err != nil {
					s.Fatalf("Finding %s tab failed: %v", cb.tab, err)
				}
				defer tabNode.Release(ctx)

				if tabNode.ClassName != "tab selected" {
					if err := tabNode.LeftClick(ctx); err != nil {
						s.Fatalf("Failed to click on %s tab: %v", cb.tab, err)
					}

					if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
						Role:      ui.RoleTypeTab,
						Name:      cb.tab,
						ClassName: "tab selected",
					}, 15*time.Second); err != nil {
						s.Fatalf("Waiting for %s tab failed: %v", cb.tab, err)
					}
				}

				if err := policyutil.VerifySettingsNode(ctx, tconn,
					ui.FindParams{
						Role: ui.RoleTypeCheckBox,
						Name: cb.name,
					},
					ui.FindParams{
						Attributes: map[string]interface{}{
							"restriction": param.wantRestriction,
							"checked":     param.wantChecked,
						},
					},
				); err != nil {
					s.Errorf("Unexpected settings state for %q: %v", cb.name, err)
				}
			}
		})
	}
}
