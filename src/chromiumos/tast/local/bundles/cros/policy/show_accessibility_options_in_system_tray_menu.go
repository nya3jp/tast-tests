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
		name string
		// show is the expected existence of the a11y button.
		show bool
		// wantRestricted is the wanted restriction state of the toggle button for the "Always show accessibility options in the system menu" option.
		wantRestricted bool
		policy         *policy.ShowAccessibilityOptionsInSystemTrayMenu
	}{
		{
			name:           "unset",
			show:           false,
			wantRestricted: false,
			policy:         &policy.ShowAccessibilityOptionsInSystemTrayMenu{Stat: policy.StatusUnset},
		},
		{
			name:           "false",
			show:           false,
			wantRestricted: true,
			policy:         &policy.ShowAccessibilityOptionsInSystemTrayMenu{Val: false},
		},
		{
			name:           "true",
			show:           true,
			wantRestricted: true,
			policy:         &policy.ShowAccessibilityOptionsInSystemTrayMenu{Val: true},
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

			// Open an empty window.
			// Opening the OS Settings page with cr.NewConn() would open it inside the browser window.
			// This does not work with UI automation, the nodes inside the page won't show up in the node tree.
			if err := kb.Accel(ctx, "ctrl+n"); err != nil {
				s.Fatal("Failed to open window: ", err)
			}

			// Type the path of the Accessibility settings page to the address bar.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "Address and search bar",
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to click address bar: ", err)
			}
			if err := kb.Type(ctx, "chrome://os-settings/osAccessibility\n"); err != nil {
				s.Fatal("Failed to open the Accessibility settings: ", err)
			}

			// Find the toggle button node.
			tbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeToggleButton,
				Name: "Always show accessibility options in the system menu",
			}, 15*time.Second)
			if err != nil {
				s.Fatal("Failed to find the toggle button: ", err)
			}
			defer tbNode.Release(ctx)

			// Check the checked state of the toggle button.
			if checked := tbNode.Checked == ui.CheckedStateTrue; checked != param.show {
				s.Logf("The checked state is %q", tbNode.Checked)
				s.Errorf("Unexpected toggle button checked state: got %t; want %t", checked, param.show)
			}

			// Check the restriction state of the toggle button.
			if restricted := tbNode.Restriction == ui.RestrictionDisabled; restricted != param.wantRestricted {
				s.Logf("The restriction state is %q", tbNode.Restriction)
				s.Errorf("Unexpected toggle button restriction: got %t; want %t", restricted, param.wantRestricted)
			}

			// Open system tray.
			if err := kb.Accel(ctx, "Alt+Shift+s"); err != nil {
				s.Fatal("Failed to press Alt+Shift+s to open system tray: ", err)
			}

			// Look for the a11y button in the system tray.
			if err := ui.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Show accessibility settings",
			}, param.show, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the Accessibility button: ", err)
			}
		})
	}
}
