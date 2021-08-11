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
			"chromeos-commercial-remote-management@google.com",
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

	// radioButtonNames is a list of UI element names in the notification settings page.
	// The order of the strings should follow the order in the settings page.
	// wantRestriction and wantChecked entries are expected to follow this order as well.
	radioButtonNames := []string{
		"Sites can ask to send notifications",
		"Use quieter messaging",
		"Don't allow sites to send notifications",
	}

	for _, param := range []struct {
		name            string
		wantPermission  string                    // the expected answer for the JS query
		wantRestriction []restriction.Restriction // the expected restriction states of the radio buttons in radioButtonNames
		wantChecked     []checked.Checked         // the expected checked states of the radio buttons in radioButtonNames
		value           *policy.DefaultNotificationsSetting
	}{
		{
			name:            "unset",
			wantPermission:  "default",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None, restriction.None},
			wantChecked:     []checked.Checked{checked.True, checked.False, checked.False},
			value:           &policy.DefaultNotificationsSetting{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantPermission:  "granted",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.True, checked.False, checked.False},
			value:           &policy.DefaultNotificationsSetting{Val: 1}, // Allow sites to show desktop notifications.
		},
		{
			name:            "deny",
			wantPermission:  "denied",
			wantRestriction: []restriction.Restriction{restriction.Disabled, restriction.Disabled, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.False, checked.False, checked.True},
			value:           &policy.DefaultNotificationsSetting{Val: 2}, // Do not allow any site to show desktop notifications.
		},
		{
			name:            "ask",
			wantPermission:  "default",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.True, checked.False, checked.False},
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

			// Check the state of the buttons.
			for i, radioButtonName := range radioButtonNames {
				if err := policyutil.CurrentPage(cr).
					SelectNode(ctx, nodewith.
						Role(role.RadioButton).
						Name(radioButtonName)).
					Restriction(param.wantRestriction[i]).
					Checked(param.wantChecked[i]).
					Verify(); err != nil {
					s.Errorf("Unexpected settings state for the %q button: %v", radioButtonName, err)
				}
			}
		})
	}
}
