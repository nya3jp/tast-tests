// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoclickEnabled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of AutoclickEnabled policy: checking if autoclick is enabled or not",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		// TODO(crbug.com/1186655): Enable test when the policy can be disabled.
		Attr:    []string{},
		Fixture: fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AutoclickEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// AutoclickEnabled tests the AutoclickEnabled policy.
func AutoclickEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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
			name:            "enabled",
			value:           &policy.AutoclickEnabled{Val: true},
			wantButton:      true,
			wantChecked:     checked.True,
			wantRestriction: restriction.Disabled,
		},
		{
			name:            "disabled",
			value:           &policy.AutoclickEnabled{Val: false},
			wantButton:      false,
			wantChecked:     checked.False,
			wantRestriction: restriction.Disabled,
		},
		{
			name:            "unset",
			value:           &policy.AutoclickEnabled{Stat: policy.StatusUnset},
			wantButton:      false,
			wantChecked:     checked.False,
			wantRestriction: restriction.None,
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
				s.Fatal("Failed to move mouse to status tray: ", err)
			}

			// Check if a click occurred by checking whether the Sign out button is visible or not.
			if err := ui.WithTimeout(time.Second * 10).WaitUntilExists(nodewith.Role(role.Window).ClassName("TrayBubbleView"))(ctx); err != nil {
				if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
					s.Fatal("Failed to wait for 'TrayBubbleView' window: ", err)
				}
				if param.wantButton {
					s.Error("Could not find 'TrayBubbleView' window")
				}
			} else {
				if !param.wantButton {
					s.Error("Unexpected 'TrayBubbleView' window found")
				}
			}

			// Move mouse to the top left corner so the next test case will have to move the mouse again.
			// Otherwise autoclick won't be triggered.
			if err := mouse.Move(tconn, coords.Point{X: 0, Y: 0}, 0)(ctx); err != nil {
				s.Fatal("Failed to move mouse to the top left corner: ", err)
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
		})
	}
}
