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
		Func: AllowDeletingBrowserHistory,
		Desc: "Behavior of AllowDeletingBrowserHistory policy, checking the correspoding checkbox states (restriction and checked) after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Pre:          pre.User,
	})
}

// AllowDeletingBrowserHistory tests the AllowDeletingBrowserHistory policy.
func AllowDeletingBrowserHistory(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name        string
		restriction ui.RestrictionState                 // restriction is the wanted restriction state of the checkboxes in Browsing history.
		wantChecked ui.CheckedState                     // wantChecked is the wanted checked state of the checkboxes in Browsing history.
		value       *policy.AllowDeletingBrowserHistory // value is the value of the policy.
	}{
		{
			name:        "unset",
			restriction: ui.RestrictionNone,
			wantChecked: ui.CheckedStateTrue,
			value:       &policy.AllowDeletingBrowserHistory{Stat: policy.StatusUnset},
		},
		{
			name:        "allow",
			restriction: ui.RestrictionNone,
			wantChecked: ui.CheckedStateTrue,
			value:       &policy.AllowDeletingBrowserHistory{Val: true},
		},
		{
			name:        "deny",
			restriction: ui.RestrictionDisabled,
			wantChecked: ui.CheckedStateFalse,
			value:       &policy.AllowDeletingBrowserHistory{Val: false},
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

			// Open settings page where the affected checkboxes can be found.
			conn, err := cr.NewConn(ctx, "chrome://settings/clearBrowserData")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Loop for different checkboxes.
			for _, cb := range []struct {
				ref  string // ref is the shortened name of the checkbox that can be used in logging.
				name string // name is a unique part of the checkbox name in the UI tree.
				tab  string // tab is the name of the tab in the UI tree that should be selected to find the checkbox.
			}{
				{
					ref:  "Browsing history",
					name: "Browsing history Clears history and autocompletions in the address bar.",
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

				// Find the checkbox node.
				cbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
					Role: ui.RoleTypeCheckBox,
					Name: cb.name,
				}, 15*time.Second)
				if err != nil {
					s.Fatalf("Finding %q checkbox failed: %v", cb.ref, err)
				}
				defer cbNode.Release(ctx)

				if isMatched, err := cbNode.MatchesParamsWithEmptyAttributes(ctx, ui.FindParams{
					Attributes: map[string]interface{}{
						"restriction": param.restriction,
						"checked":     param.wantChecked,
					},
				}); err != nil {
					s.Fatal("Failed to check a matching node: ", err)
				} else if isMatched == false {
					s.Errorf("Failed to verify the matching checkbox node; got (%#v, %#v), want (%#v, %#v)", cbNode.Checked, cbNode.Restriction, param.wantChecked, param.restriction)
				}
			}
		})
	}
}
