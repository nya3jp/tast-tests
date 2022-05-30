// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/chromiumos/tast-tests/common/fixture"
	"go.chromium.org/chromiumos/tast-tests/common/policy"
	"go.chromium.org/chromiumos/tast-tests/common/policy/fakedms"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/checked"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/mouse"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/restriction"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/coords"
	"go.chromium.org/chromiumos/tast-tests/local/policyutil"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoclickEnabled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of AutoclickEnabled policy: checking if autoclick is enabled or not",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		// TODO(crbug.com/1186655): Enable test when the policy can be disabled.
		Attr:    []string{},
		Fixture: fixture.ChromePolicyLoggedIn,
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
