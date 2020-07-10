// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/policyutil"
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
		Pre:          pre.User,
	})
}

// AllowDeletingBrowserHistory tests the AllowDeletingBrowserHistory policy.
func AllowDeletingBrowserHistory(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, param := range []struct {
		name           string
		wantRestricted bool                                // wantRestricted is the wanted state of the Browsing history checkbox.
		wantChecked    bool                                // wantChecked is the wanted checked state of the checkboxes in Browsing history.
		value          *policy.AllowDeletingBrowserHistory // value is the value of the policy.
	}{
		{
			name:           "unset",
			wantRestricted: false,
			wantChecked:    true,
			value:          &policy.AllowDeletingBrowserHistory{Stat: policy.StatusUnset},
		},
		{
			name:           "allow",
			wantRestricted: false,
			wantChecked:    true,
			value:          &policy.AllowDeletingBrowserHistory{Val: true},
		},
		{
			name:           "deny",
			wantRestricted: true,
			wantChecked:    false,
			value:          &policy.AllowDeletingBrowserHistory{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Connect to Test API to use it with the ui library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// Open settings page where the affected checkboxes can be found.
			conn, err := cr.NewConn(ctx, "chrome://settings/clearBrowserData")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Loop for different checkboxes.
			for _, cb := range []struct {
				name string // name is a unique part of the checkbox name in the UI tree.
				tab  string // tab is the name of the tab in the UI tree that should be selected to find the cehckbox.
			}{
				{
					name: "Browsing history Clears history and autocompletions in the address bar.",
					tab:  "Basic",
				},
				{
					name: "Browsing history None",
					tab:  "Advanced",
				},
				{
					name: "Download history None",
					tab:  "Advanced",
				},
			} {
				// Select the tab if it not selected already.
				tabNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
					Role: ui.RoleTypeTab,
					Name: cb.tab,
				}, 10*time.Second)
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
					}, 10*time.Second); err != nil {
						s.Fatalf("Waiting for %s tab failed: %v", cb.tab, err)
					}
				}

				// Find the checkbox node.
				cbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
					Role: ui.RoleTypeCheckBox,
					Name: cb.name,
				}, 10*time.Second)
				if err != nil {
					s.Fatalf("Finding %s checkbox failed: %v", cb.name, err)
				}
				defer cbNode.Release(ctx)

				// Check the checked state of the checkbox.
				checked, err := cbNode.Attribute(ctx, "checked")
				if err != nil {
					s.Fatalf("Failed to get the checked attribute of the %s checkbox in the %s tab: %v", cb.name, cb.tab, err)
				}

				checkedStr, ok := checked.(string)
				if !ok {
					s.Fatalf("The checked attribute of the %s checkbox is not a string", cb.name)
				}

				if checkedStr != strconv.FormatBool(param.wantChecked) {
					s.Errorf("Unexpected %s checkbox checked state in the %s tab: got %t; want %t", cb.name, cb.tab, checkedStr, param.wantChecked)
				}

				// Check the restriction setting of the checkbox.
				if restriction, err := cbNode.Attribute(ctx, "restriction"); err == nil {
					restrictionStr, ok := restriction.(string)
					if !ok {
						s.Fatalf("The restriction attribute of the %s checkbox is not a string", cb.name)
					}
					if (restrictionStr == "disabled" || restrictionStr == "readOnly") != param.wantRestricted {
						s.Errorf("Unexpected %s checkbox restriction in the %s tab: want %t. Correspondig restriction string: %s", cb.name, cb.tab, param.wantRestricted, restrictionStr)
					}

					s.Logf("The restriction is %s", restrictionStr)
				} else {
					// The restriction attribute is optional. If it does not exists, the checkbox is not restricted.
					if param.wantRestricted {
						s.Errorf("Unexpected %s checkbox restriction in the %s tab: got false; want %t", cb.name, cb.tab, param.wantRestricted)
					}
				}
			}
		})
	}

}
