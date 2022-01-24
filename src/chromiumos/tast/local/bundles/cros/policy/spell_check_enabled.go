// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SpellCheckEnabled,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that the SpellCheckEnabled policy is correctly reflected on the Chrome Settings page",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
	})
}

func SpellCheckEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name  string
		value *policy.SpellcheckEnabled
	}{
		{
			name:  "enabled",
			value: &policy.SpellcheckEnabled{Val: true},
		},
		{
			name:  "disabled",
			value: &policy.SpellcheckEnabled{Val: false},
		},
		{
			name:  "unset",
			value: &policy.SpellcheckEnabled{Stat: policy.StatusUnset},
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

			// Open lacros browser.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}

			// Open Language settings page.
			if err := conn.Navigate(ctx, "chrome://settings/languages"); err != nil {
				s.Fatal("Failed to open Language settings page: ", err)
			}
			defer conn.Close()

			// Create a uiauto.Context with default timeout.
			ui := uiauto.New(tconn)

			spellCheckFinder := nodewith.Name("Spell check").Role(role.ToggleButton)
			if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(spellCheckFinder)(ctx); err != nil {
				s.Fatal("Failed to find Spell check toggle: ", err)
			}

			var info *uiauto.NodeInfo
			if info, err = ui.WithTimeout(5*time.Second).Info(ctx, spellCheckFinder); err != nil {
				s.Fatal("Failed to get node info for Spell check toggle: ", err)
			}

			if param.value.Stat != policy.StatusSet {
				// Policy is unset, button should be enabled.
				if info.Restriction != restriction.None {
					b, _ := json.Marshal(info)
					s.Fatalf("Unexpected Spell check toggle state, expected enabled, got %s", b)
				}
			} else if param.value.Val {
				// Policy is enabled, button should be disabled and checked.
				if info.Checked != checked.True || info.Restriction != restriction.Disabled {
					b, _ := json.Marshal(info)
					s.Fatalf("Unexpected Spell check toggle state, expected checked and disabled, got %s", b)
				}
			} else {
				// Policy is disabled, button should be disabled and unchecked.
				if info.Checked != checked.False || info.Restriction != restriction.Disabled {
					b, _ := json.Marshal(info)
					s.Fatalf("Unexpected Spell check toggle state, expected unchecked and disabled, got %s", b)
				}
			}
		})
	}
}
