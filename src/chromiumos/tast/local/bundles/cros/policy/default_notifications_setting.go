// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto/checked"
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
		Func: DefaultNotificationsSetting,
		Desc: "Behavior of DefaultNotificationsSetting policy, checks the notification permission in JavaScript at different policy values",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// DefaultNotificationsSetting tests the DefaultNotificationsSetting policy.
func DefaultNotificationsSetting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, param := range []struct {
		name            string
		wantPermission  string                  // the expected answer for the JS query
		wantRestriction restriction.Restriction // the expected restriction state of the toggle button
		wantChecked     checked.Checked         // the expected checked state of the toggle button
		value           *policy.DefaultNotificationsSetting
	}{
		{
			name:            "unset",
			wantPermission:  "default",
			wantRestriction: restriction.None,
			wantChecked:     checked.True,
			value:           &policy.DefaultNotificationsSetting{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantPermission:  "granted",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.True,
			value:           &policy.DefaultNotificationsSetting{Val: 1}, // Allow sites to show desktop notifications.
		},
		{
			name:            "deny",
			wantPermission:  "denied",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.False,
			value:           &policy.DefaultNotificationsSetting{Val: 2}, // Do not allow any site to show desktop notifications.
		},
		{
			name:            "ask",
			wantPermission:  "default",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.True,
			value:           &policy.DefaultNotificationsSetting{Val: 3}, // Ask every time a site wants to show desktop notifications.
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

			// Check the button states.
			if err := policyutil.CurrentPage(cr).
				SelectNode(ctx, nodewith.
					Role(role.ToggleButton).
					Name("Sites can ask to send notifications")).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}
