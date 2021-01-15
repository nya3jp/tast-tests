// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AutoclickEnabled,
		Desc: "Behavior of AutoclickEnabled policy: checking if autoclick is enabled or not",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// AutoclickEnabled tests the AutoclickEnabled policy.
func AutoclickEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	for _, param := range []struct {
		name            string
		value           *policy.AutoclickEnabled
		wantButton      bool
		wantChecked     ui.CheckedState
		wantRestriction ui.RestrictionState
	}{
		{
			name:            "unset",
			value:           &policy.AutoclickEnabled{Stat: policy.StatusUnset},
			wantButton:      false,
			wantChecked:     ui.CheckedStateFalse,
			wantRestriction: ui.RestrictionNone,
		},
		{
			name:            "disabled",
			value:           &policy.AutoclickEnabled{Val: false},
			wantButton:      false,
			wantChecked:     ui.CheckedStateFalse,
			wantRestriction: ui.RestrictionDisabled,
		},
		{
			name:            "enabled",
			value:           &policy.AutoclickEnabled{Val: true},
			wantButton:      true,
			wantChecked:     ui.CheckedStateTrue,
			wantRestriction: ui.RestrictionDisabled,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Find the system tray button node.
			stNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role:      ui.RoleTypeButton,
				ClassName: "UnifiedSystemTray",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Failed to find the system tray button: ", err)
			}
			defer stNode.Release(ctx)

			// Move mouse to the middle of the system tray button.
			c := coords.Point{
				X: stNode.Location.Left + stNode.Location.Width/2,
				Y: stNode.Location.Top + stNode.Location.Height/2,
			}

			if err := mouse.Move(ctx, tconn, c, 0); err != nil {
				s.Fatal("Failed to move the mouse: ", err)
			}

			// Check if a click occurred by checking whether the Sign out button is visible or not.
			if err := ui.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Name:      "Sign out",
				ClassName: "SignOutButton",
			}, param.wantButton, 30*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the Sign out button: ", err)
			}

			// Open settings page where the affected toggle button can be found.
			sconn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osAccessibility")
			if err != nil {
				s.Fatal("Failed to connect to the accessibility settings page: ", err)
			}
			defer sconn.Close()

			// Find and click manage accessibility link.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeLink,
				Name: "Manage accessibility features Enable accessibility features",
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to find and click Manage accessibility features link: ", err)
			}

			if err := policyutil.VerifySettingsNode(ctx, tconn,
				ui.FindParams{
					Role: ui.RoleTypeToggleButton,
					Name: "Automatically click when the cursor stops",
				},
				ui.FindParams{
					Attributes: map[string]interface{}{
						"restriction": param.wantRestriction,
						"checked":     param.wantChecked,
					},
				},
			); err != nil {
				s.Error("Unexpected settings state: ", err)
			}

			if param.wantChecked == ui.CheckedStateTrue {
				// Policy unset will open the modal dialog about turning off autoclicks.
				// We need to close it. Otherwise it messes up the next test.

				if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
					s.Fatal("Failed to clean up: ", err)
				}

				// Find Yes button on the dialog.
				yesButton, err := ui.StableFind(ctx, tconn, ui.FindParams{
					ClassName: "MdTextButton",
					Name:      "Yes",
				}, &testing.PollOptions{Timeout: 15 * time.Second})
				if err != nil {
					s.Fatal("Failed to find and yes button: ", err)
				}

				condition := func(ctx context.Context) (bool, error) {
					isDialogShown, err := ui.Exists(ctx, tconn, ui.FindParams{
						Name: "Are you sure you want to turn off automatic clicks?",
					})
					return !isDialogShown, err
				}

				opts := testing.PollOptions{Timeout: 15 * time.Second, Interval: 500 * time.Millisecond}
				// Click until the dialog is gone.
				if err := yesButton.LeftClickUntil(ctx, condition, &opts); err != nil {
					s.Fatal("Failed to close the dialog: ", err)
				}
			}
		})
	}
}
