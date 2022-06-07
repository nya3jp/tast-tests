// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BrowserPolicy,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Ensures policies are used by browser for unicorn users via Devtools Enabled policy",
		Contacts: []string{
			"lamzin@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: "familyLinkUnicornPolicyLogin",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           "familyLinkUnicornPolicyLoginWithLacros",
			Val:               browser.TypeLacros,
		}},
		Timeout: 4 * time.Minute,
	})
}

func BrowserPolicy(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*familylink.FixtData).FakeDMS
	cr := s.FixtValue().(*familylink.FixtData).Chrome

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

			// TODO(crbug.com/1259615): This should be part of the fixture.
			_, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			for _, keys := range []string{
				"Ctrl+Shift+C",
				"Ctrl+Shift+I",
				"F12",
				"Ctrl+Shift+J",
			} {
				s.Run(ctx, keys, func(ctx context.Context, s *testing.State) {
					defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, s.OutDir(), s.HasError, tconn, fmt.Sprintf("ui_tree_%s_%s.txt", tc.name, keys))

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
					ui := uiauto.New(tconn)
					userAction := nodewith.Name("User Action").Role(role.ColumnHeader)
					if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(userAction)(ctx); err != nil {
						s.Fatal("Failed to wait for page nodes: ", err)
					}
					// Press keys combination to open DevTools.
					if err := keyboard.Accel(ctx, keys); err != nil {
						s.Fatalf("Failed to press %s: %v", keys, err)
					}
					timeout := 15 * time.Second
					elements := nodewith.Name("Elements").Role(role.Tab)
					if tc.wantAllowed {
						if err := ui.WithTimeout(timeout).WaitUntilExists(elements)(ctx); err != nil {
							s.Error("Failed to wait for DevTools: ", err)
						}
					} else {
						if err := policyutil.VerifyNotExists(ctx, tconn, elements, timeout); err != nil {
							s.Errorf("Failed to verify that DevTools are not available after %s: %s", timeout, err)
						}
					}
				})
			}
		})
	}
}
