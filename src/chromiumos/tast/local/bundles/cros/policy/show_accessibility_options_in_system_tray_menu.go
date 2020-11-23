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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShowAccessibilityOptionsInSystemTrayMenu,
		Desc: "Behavior of ShowAccessibilityOptionsInSystemTrayMenu policy: check the a11y option in the system tray, and the status of the related option in the settings",
		Contacts: []string{
			"gabormagda@goggle.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.User,
	})
}

// ShowAccessibilityOptionsInSystemTrayMenu tests the ShowAccessibilityOptionsInSystemTrayMenu policy.
func ShowAccessibilityOptionsInSystemTrayMenu(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Open a keyboard device.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	for _, param := range []struct {
		name            string
		wantChecked     ui.CheckedState     // wantChecked is the expected existence of the a11y button.
		wantRestriction ui.RestrictionState // wantRestriction is the wanted restriction state of the toggle button for the "Always show accessibility options in the system menu" option.
		policy          *policy.ShowAccessibilityOptionsInSystemTrayMenu
	}{
		{
			name:            "unset",
			wantChecked:     ui.CheckedStateFalse,
			wantRestriction: ui.RestrictionNone,
			policy:          &policy.ShowAccessibilityOptionsInSystemTrayMenu{Stat: policy.StatusUnset},
		},
		{
			name:            "false",
			wantChecked:     ui.CheckedStateFalse,
			wantRestriction: ui.RestrictionDisabled,
			policy:          &policy.ShowAccessibilityOptionsInSystemTrayMenu{Val: false},
		},
		{
			name:            "true",
			wantChecked:     ui.CheckedStateTrue,
			wantRestriction: ui.RestrictionDisabled,
			policy:          &policy.ShowAccessibilityOptionsInSystemTrayMenu{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open settings page where the affected toggle button can be found.
			if err := policyutil.VerifySettingsState(ctx, cr, "chrome://os-settings/osAccessibility",
				ui.FindParams{
					Role: ui.RoleTypeToggleButton,
					Name: "Always show accessibility options in the system menu",
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

			// Open system tray.
			if err := kb.Accel(ctx, "Alt+Shift+s"); err != nil {
				s.Fatal("Failed to press Alt+Shift+s to open system tray: ", err)
			}

			// Look for the a11y button in the system tray.
			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Show accessibility settings",
			}, param.wantChecked == ui.CheckedStateTrue, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the Accessibility button: ", err)
			}
		})
	}
}
