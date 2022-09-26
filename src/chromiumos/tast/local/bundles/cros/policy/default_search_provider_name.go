// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
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
		Func:         DefaultSearchProviderName,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of DefaultSearchProviderName policy: check if specified provider name is displayed correctly",
		Contacts: []string{
			"jaflis@google.org", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
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
			pci.SearchFlag(&policy.DefaultSearchProviderKeyword{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.DefaultSearchProviderEnabled{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.DefaultSearchProviderName{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.DefaultSearchProviderSearchURL{}, pci.VerifiedValue),
		},
	})
}

func DefaultSearchProviderName(ctx context.Context, s *testing.State) {
	const (
		testSearchProviderName     = "testProvider"
		testSearchProviderHostname = "test-provider"
		testKeyword                = "test-keyword"
	)

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

	uiauto := uiauto.New(tconn)

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar")

	for _, param := range []struct {
		name                       string                            // name is the subtest name.
		value                      *policy.DefaultSearchProviderName // value is the policy value.
		expectedSearchProviderName string                            // the expected name of the search provider
	}{
		{
			name:                       "set",
			value:                      &policy.DefaultSearchProviderName{Val: testSearchProviderName},
			expectedSearchProviderName: testSearchProviderName,
		},
		{
			// Leaving DefaultSearchProviderName unset means the hostname specified by the search URL is used
			name:                       "unset",
			value:                      &policy.DefaultSearchProviderName{Stat: policy.StatusUnset},
			expectedSearchProviderName: testSearchProviderHostname,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			// - DefaultSearchProviderName is applied only when DefaultSearchProviderEnabled is on.
			// - Value of DefaultSearchProviderSearchURL is used as a fallback, when DefaultSearchProviderName is not set.
			// - DefaultSearchProviderName is passed as a parameter.
			policies := []policy.Policy{
				&policy.DefaultSearchProviderEnabled{Val: true},
				&policy.DefaultSearchProviderKeyword{Val: testKeyword},
				&policy.DefaultSearchProviderSearchURL{Val: fmt.Sprintf("https://%s/search?q={searchTerms}", testSearchProviderHostname)},
				param.value}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open an empty page.
			// Use chrome://newtab to open new tab page (see https://crbug.com/1188362#c19).
			conn, err := br.NewConn(ctx, "chrome://newtab/")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Click the address and search bar.
			if err := uiauto.LeftClick(addressBarNode)(ctx); err != nil {
				s.Fatal("Could not find the address bar: ", err)
			}

			// Type the keyword.
			if err := kb.Type(ctx, testKeyword); err != nil {
				s.Fatal("Failed to write keyword event: ", err)
			}

			// Press Tab to trigger the search provider.
			if err := kb.Accel(ctx, "Tab"); err != nil {
				s.Fatal("Failed to write Tab event: ", err)
			}

			// Wait for UI elements containing the name of the search provider to appear.
			if err := uiauto.WaitUntilExists(nodewith.Name("Search " + param.expectedSearchProviderName).
				ClassName("SelectedKeywordView"))(ctx); err != nil {
				s.Fatal("Failed to wait for the search provider button to appear: ", err)
			}

			// We're looking for a node labeled "Search mode, press Enter to search <PROVIDER_NAME>".
			// This is a long string and it may change in the future, so we ignore everything but the actual provider name.
			if err := uiauto.WaitUntilExists(nodewith.NameContaining(param.expectedSearchProviderName).
				ClassName("OmniboxSuggestionRowButton").Role(role.ListBoxOption))(ctx); err != nil {
				s.Fatal("Failed to wait for Omnibox suggestion button to appear: ", err)
			}
		})
	}
}
