// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
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
		Func: AutoclickEnabled,
		Desc: "Behavior of AutoclickEnabled policy: checking if autoclick is enabled or not",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// AutoclickEnabled tests the AutoclickEnabled policy.
func AutoclickEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name            string
		value           *policy.AutoclickEnabled
		wantButton      bool
		wantChecked     checked.Checked
		wantRestriction restriction.Restriction
	}{
		{
			name:            "unset",
			value:           &policy.AutoclickEnabled{Stat: policy.StatusUnset},
			wantButton:      false,
			wantChecked:     checked.False,
			wantRestriction: restriction.None,
		},
		{
			name:            "disabled",
			value:           &policy.AutoclickEnabled{Val: false},
			wantButton:      false,
			wantChecked:     checked.False,
			wantRestriction: restriction.Disabled,
		},
		{
			name:            "enabled",
			value:           &policy.AutoclickEnabled{Val: true},
			wantButton:      true,
			wantChecked:     checked.True,
			wantRestriction: restriction.Disabled,
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

			ui := uiauto.New(tconn)

			// Move mouse to status tray.
			if err := ui.MouseMoveTo(nodewith.ClassName("ash/StatusAreaWidgetDelegate"), 0)(ctx); err != nil {
				s.Fatal("Failed to move mouse to status tray")
			}

			// Check if a click occurred by checking whether the Sign out button is visible or not.
			if err := ui.WithTimeout(time.Second * 10).WaitUntilExists(nodewith.Name("Sign out").ClassName("SignOutButton"))(ctx); err != nil {
				if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
					s.Fatal("Failed to wait for 'Sign out' button: ", err)
				}
				if param.wantButton {
					s.Error("Could not find 'Sign out' button")
				}
			} else {
				if !param.wantButton {
					s.Error("Unexpected 'Sign out' button found")
				}
			}

			// Move mouse to Launcher button so the next test case will have to move the mouse again.
			// Otherwise autoclick won't be triggered.
			if err := ui.MouseMoveTo(nodewith.Name("Launcher").ClassName("ash/HomeButton"), 0)(ctx); err != nil {
				s.Fatal("Failed to move mouse to status tray")
			}

			if err := policyutil.OSSettingsPage(ctx, cr, "manageAccessibility").
				SelectNode(ctx, nodewith.
					Name("Automatically click when the cursor stops").
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}

			if param.wantChecked == checked.True {
				// Policy unset will open the modal dialog about turning off autoclicks.
				// We need to close it. Otherwise it messes up the next test.

				if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
					s.Fatal("Failed to clean up: ", err)
				}

				// TODO(crbug.com/1197511): investigate why this is needed.
				// Wait for a second before clicking the Yes button as the click
				// won't be registered otherwise.
				testing.Sleep(ctx, time.Second)

				condition := func(ctx context.Context) error {
					return ui.Gone(nodewith.Name("Are you sure you want to turn off automatic clicks?"))(ctx)
				}

				// Click until the dialog is gone.
				if err := ui.LeftClickUntil(nodewith.Name("Yes").ClassName("MdTextButton"), condition)(ctx); err != nil {
					s.Fatal("Failed to close the dialog: ", err)
				}
			}
		})
	}
}
