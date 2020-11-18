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
		Func: DefaultNotificationsSetting,
		Desc: "Behavior of DefaultNotificationsSetting policy, checks the notification permission in JavaScript at different policy values",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// DefaultNotificationsSetting tests the DefaultNotificationsSetting policy.
func DefaultNotificationsSetting(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name           string
		wantPermission string          // the expected answer for the JS query
		wantRestricted bool            // the expected restriction state of the toggle button
		wantChecked    ui.CheckedState // the expected checked state of the toggle button
		value          *policy.DefaultNotificationsSetting
	}{
		{
			name:           "unset",
			wantPermission: "default",
			wantRestricted: false,
			wantChecked:    ui.CheckedStateTrue,
			value:          &policy.DefaultNotificationsSetting{Stat: policy.StatusUnset},
		},
		{
			name:           "allow",
			wantPermission: "granted",
			wantRestricted: true,
			wantChecked:    ui.CheckedStateTrue,
			value:          &policy.DefaultNotificationsSetting{Val: 1}, // Allow sites to show desktop notifications.
		},
		{
			name:           "deny",
			wantPermission: "denied",
			wantRestricted: true,
			wantChecked:    ui.CheckedStateFalse,
			value:          &policy.DefaultNotificationsSetting{Val: 2}, // Do not allow any site to show desktop notifications.
		},
		{
			name:           "ask",
			wantPermission: "default",
			wantRestricted: true,
			wantChecked:    ui.CheckedStateTrue,
			value:          &policy.DefaultNotificationsSetting{Val: 3}, // Ask every time a site wants to show desktop notifications.
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

			// Open notification settings.
			conn, err := cr.NewConn(ctx, "chrome://settings/content/notifications")
			if err != nil {
				s.Fatal("Failed to open notification settings: ", err)
			}
			defer conn.Close()

			var permission string
			if err := conn.Eval(ctx, "Notification.permission", &permission); err != nil {
				s.Fatal("Failed to eval: ", err)
			} else if permission != param.wantPermission {
				s.Errorf("Unexpected permission value; got %s, want %s", permission, param.wantPermission)
			}

			tbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeToggleButton,
				Name: "Sites can ask to send notifications",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Finding toggle button failed: ", err)
			}
			defer tbNode.Release(ctx)

			// Check the checked state of the toggle button.
			if tbNode.Checked != param.wantChecked {
				s.Errorf("Unexpected toggle button checked state: got %v; want %v", tbNode.Checked, param.wantChecked)
			}

			// Check the restriction setting of the toggle button.
			if restricted := (tbNode.Restriction == ui.RestrictionDisabled || tbNode.Restriction == ui.RestrictionReadOnly); restricted != param.wantRestricted {
				s.Logf("The restriction state is %q", tbNode.Restriction)
				s.Errorf("Unexpected toggle button restriction: got %t; want %t", restricted, param.wantRestricted)
			}
		})
	}
}
