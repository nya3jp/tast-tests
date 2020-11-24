// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
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
		Func: DeveloperToolsAvailability,
		// TODO(crbug/1125548): add functionality to verify policy with force installed extension.
		Desc: "Behavior of the DeveloperToolsAvailability policy, check whether developer tools can be opened on chrome://user-actions page",
		Contacts: []string{
			"lamzin@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func DeveloperToolsAvailability(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

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

			// Open new tab and navigate to chrome://user-actions.
			// Here we cannot use cr.Conn, because Chrome DevTools Protocol
			// relies on DevTools.
			if err := keyboard.Accel(ctx, "Ctrl+T"); err != nil {
				s.Fatal("Failed to press Ctrl+T: ", err)
			}
			if err := keyboard.Type(ctx, "chrome://user-actions\n"); err != nil {
				s.Fatal("Failed to type chrome://user-actions: ", err)
			}

			for _, keys := range []string{
				"Ctrl+Shift+C",
				"Ctrl+Shift+I",
				"F12",
				"Ctrl+Shift+J",
			} {
				s.Run(ctx, keys, func(ctx context.Context, s *testing.State) {
					defer func(ctx context.Context) {
						// Attempt to close DevTools and reload page.
						if err := keyboard.Accel(ctx, "F12"); err != nil {
							s.Fatal("Failed to press F12: ", err)
						}
						if err := keyboard.Accel(ctx, "Ctrl+R"); err != nil {
							s.Fatal("Failed to press Ctrl+R: ", err)
						}
					}(ctx)

					defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, fmt.Sprintf("ui_tree_%s_%s.txt", tc.name, keys))

					// Press keys combination to open DevTools.
					if err := keyboard.Accel(ctx, keys); err != nil {
						s.Fatalf("Failed to press %s: %v", keys, err)
					}

					// Check that we have access to chrome://user-actions accessability tree.
					if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
						Name: "User Action",
						Role: ui.RoleTypeColumnHeader,
					}, 5*time.Second); err != nil {
						s.Fatal("Failed to wait for page nodes: ", err)
					}

					elementsParams := ui.FindParams{Name: "Elements", Role: ui.RoleTypeTab}

					switch tc.wantAllowed {
					case false:
						s.Log("Sleep to give a chance for DevTools to appear if policy does not work correctly")
						if err := testing.Sleep(ctx, 5*time.Second); err != nil {
							s.Fatal("Failed to sleep: ", err)
						}
						if exists, err := ui.Exists(ctx, tconn, elementsParams); err != nil {
							s.Fatal("Failed to check whether DevTools are available: ", err)
						} else if exists {
							s.Error("Unexpected DevTools availability: get allowed; want disallowed")
						}
					case true:
						if err := ui.WaitUntilExists(ctx, tconn, elementsParams, 5*time.Second); err != nil {
							s.Error("Failed to wait for DevTools: ", err)
						}
					}
				})
			}
		})
	}
}
