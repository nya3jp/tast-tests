// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeveloperToolsAvailability,
		// TODO(crbug/1125548): add functionality to verify policy with force installed extension.
		Desc: "Behavior of the DeveloperToolsAvailability policy, check whether developer tools can be opened on chrome://user-actions page",
		Contacts: []string{
			"lamzin@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

func DeveloperToolsAvailability(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	for _, tc := range []struct {
		name        string
		value       []policy.Policy
		wantAllowed bool
	}{
		{
			name:        "disallowed_for_force_installed_extensions",
			value:       []policy.Policy{&policy.DeveloperToolsAvailability{Val: 0}},
			wantAllowed: true,
		},
		{
			name:        "alowed",
			value:       []policy.Policy{&policy.DeveloperToolsAvailability{Val: 1}},
			wantAllowed: true,
		},
		{
			name:        "disallowed",
			value:       []policy.Policy{&policy.DeveloperToolsAvailability{Val: 2}},
			wantAllowed: false,
		},
		{
			name:        "unset",
			value:       []policy.Policy{},
			wantAllowed: true,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.value); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			for _, keys := range []string{
				"Ctrl+Shift+C",
				"Ctrl+Shift+I",
				"F12",
				"Ctrl+Shift+J",
			} {
				s.Run(ctx, keys, func(ctx context.Context, s *testing.State) {
					defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, fmt.Sprintf("ui_tree_%s_%s.txt", tc.name, keys))

					// Open new tab and navigate to chrome://user-actions.
					// Here we cannot use cr.Conn, because Chrome DevTools Protocol
					// relies on DevTools.
					if err := keyboard.Accel(ctx, "Ctrl+T"); err != nil {
						s.Fatal("Failed to press Ctrl+T: ", err)
					}
					if err := keyboard.Type(ctx, "chrome://user-actions\n"); err != nil {
						s.Fatal("Failed to type chrome://user-actions: ", err)
					}

					// Check that we have access to chrome://user-actions accessability tree.
					ui := uiauto.New(tconn).WithTimeout(5 * time.Second)
					params := nodewith.Name("User Action").Role(role.ColumnHeader)
					if err := ui.WaitUntilExists(params)(ctx); err != nil {
						s.Fatal("Failed to wait for page nodes: ", err)
					}

					// Press keys combination to open DevTools.
					if err := keyboard.Accel(ctx, keys); err != nil {
						s.Fatalf("Failed to press %s: %v", keys, err)
					}

					timeout := 5 * time.Second
					//elementsParams := ui.FindParams{Name: "Elements", Role: ui.RoleTypeTab}
					elementsParams := nodewith.Name("Elements").Role(role.Tab)

					switch tc.wantAllowed {
					case false:
						if err := policyutil.UiautoVerifyNotExists(ctx, tconn, elementsParams, timeout); err != nil {
							s.Errorf("Failed to verify that DevTools are not available after %s: %s", timeout, err)
						}
					case true:
						ui := uiauto.New(tconn).WithTimeout(timeout)
						if err := ui.WaitUntilExists(elementsParams)(ctx); err != nil {
							s.Error("Failed to wait for DevTools: ", err)
						}
					}
				})
			}
		})
	}
}
