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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
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
		Func:         SearchSuggestEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of SearchSuggestEnabled policy, check if a search suggestions are shown based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:commercial_limited"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.SearchSuggestEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func SearchSuggestEnabled(ctx context.Context, s *testing.State) {
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

	// Open a keyboard device.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

	for _, param := range []struct {
		name    string
		enabled bool                         // enabled is the expected enabled state of the virtual keyboard.
		policy  *policy.SearchSuggestEnabled // policy is the policy we test.
	}{
		{
			name:    "unset",
			enabled: true,
			policy:  &policy.SearchSuggestEnabled{Stat: policy.StatusUnset},
		},
		{
			name:    "disabled",
			enabled: false,
			policy:  &policy.SearchSuggestEnabled{Val: false},
		},
		{
			name:    "enabled",
			enabled: true,
			policy:  &policy.SearchSuggestEnabled{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Try to open a tab.
			if err := keyboard.Accel(ctx, "ctrl+t"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Click the address bar.
			addressBar := nodewith.Name("Address and search bar").Role(role.TextField)
			ui := uiauto.New(tconn)
			if err := uiauto.Combine("find and click the address bar",
				ui.WaitUntilExists(addressBar),
				ui.LeftClick(addressBar),
			)(ctx); err != nil {
				s.Fatal("Failed to find and click the address bar: ", err)
			}

			// Wait for a second before typing to make sure the module for
			// suggestions is loaded.
			testing.Sleep(ctx, time.Second)

			// Type something so suggestions pop up.
			if err := keyboard.Type(ctx, "google"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Wait for the omnibox popup node.
			if err := ui.WaitUntilExists(nodewith.ClassName("OmniboxPopupContentsView"))(ctx); err != nil {
				s.Fatal("Failed to find omnibox popup: ", err)
			}

			// Get all the omnibox results.
			omniboxResults, err := ui.NodesInfo(ctx, nodewith.ClassName("OmniboxResultView"))
			if err != nil {
				s.Fatal("Failed to get omnibox results: ", err)
			}

			suggest := false
			for _, result := range omniboxResults {
				if strings.Contains(result.Name, "search suggestion") {
					suggest = true
					break
				}
			}

			if suggest != param.enabled {
				s.Errorf("Unexpected existence of search suggestions: got %t; want %t", suggest, param.enabled)
			}
		})
	}
}
