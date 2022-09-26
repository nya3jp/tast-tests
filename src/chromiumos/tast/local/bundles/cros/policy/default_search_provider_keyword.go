// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultSearchProviderKeyword,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of DefaultSearchProviderKeyword policy: check if specified keyword triggers the search for search provider",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
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
			pci.SearchFlag(&policy.DefaultSearchProviderKeyword{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.DefaultSearchProviderEnabled{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.DefaultSearchProviderSearchURL{}, pci.VerifiedValue),
		},
	})
}

func DefaultSearchProviderKeyword(ctx context.Context, s *testing.State) {
	const (
		testSearchEngine = "fakeurl"      // testSearchEngine is used as fake search engine.
		testKeyword      = "tranquillity" // testKeyword is used as keyword that triggers the search engine.
		testSearchTerm   = "vy6ys"        // testSearchTerm is a value for test search.
	)
	addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar")

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

	for _, param := range []struct {
		name    string                               // name is the subtest name.
		enabled bool                                 // enabled is the expected enabled state of the policy.
		value   *policy.DefaultSearchProviderKeyword // value is the policy value.
	}{
		{
			name:    "set",
			enabled: true,
			value:   &policy.DefaultSearchProviderKeyword{Val: testKeyword},
		},
		{
			name:    "unset",
			enabled: false,
			// Leaving DefaultSearchProviderKeyword unset means no keyword activates the search provider.
			value: &policy.DefaultSearchProviderKeyword{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			// - DefaultSearchProviderKeyword can specify the keyword only when DefaultSearchProviderEnabled is on.
			// - Use DefaultSearchProviderSearchURL to set testing search engine.
			policies := []policy.Policy{
				&policy.DefaultSearchProviderEnabled{Val: true},
				&policy.DefaultSearchProviderSearchURL{Val: fmt.Sprintf("https://%s/search?q={searchTerms}", testSearchEngine)},
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

			// Connect to Test API of the used browser to clear the browser
			// history. We need a second connection as the clearing of the
			// history has to be executed from the used browser while the
			// uiauto package needs a connection to the ash browser.
			tconn2, err := br.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// Clear the browser history, otherwise the previous search results can
			// interfere with the test.
			if err := tconn2.Eval(ctx, `tast.promisify(chrome.browsingData.removeHistory({"since": 0}))`, nil); err != nil {
				s.Fatal("Failed to clear browsing history: ", err)
			}

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

			// Type something to do the search.
			if err := kb.Type(ctx, testSearchTerm+"\n"); err != nil {
				s.Fatal("Failed to type search term: ", err)
			}

			// Wait for the page to load.
			if err := uiauto.WaitForLocation(addressBarNode)(ctx); err != nil {
				s.Fatal("Failed to wait for location change: ", err)
			}

			// Find the address bar.
			nodeInfo, err := uiauto.Info(ctx, addressBarNode)
			if err != nil {
				s.Fatal("Could not get new info for the address bar: ", err)
			}
			location := nodeInfo.Value

			// Check that test search engine was triggered by the keyword and '{searchTerms}' was replaced in the query by the user's search term.
			keywordPresent := strings.Contains(location, fmt.Sprintf("%s/search?q=%s", testSearchEngine, testSearchTerm))
			if param.enabled != keywordPresent {
				s.Fatalf("Unexpected usage of the search engine keyword: got %t; want %t", keywordPresent, param.enabled)
			}
		})
	}
}
