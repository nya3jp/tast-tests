// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultSearchProviderKeyword,
		Desc: "Behavior of DefaultSearchProviderKeyword policy: check if specified keyword triggers the search for search provider",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DefaultSearchProviderKeyword(ctx context.Context, s *testing.State) {
	const (
		testSearchEngine = "fakeurl"      // testSearchEngine is used as fake search engine.
		testKeyword      = "tranquillity" // testKeyword is used as keyword that triggers the search engine.
		testSearchTerm   = "abc"          // testSearchTerm is a value for test search.
	)
	addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar")

	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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

			// Clear the browser history.
			if err := tconn.Eval(ctx, `tast.promisify(chrome.browsingData.removeHistory({"since": 0}))`, nil); err != nil {
				s.Fatal("Failed to clear browsing history: ", err)
			}

			// Open an empty page.
			// Use chrome://newtab to open new tab page (see https://crbug.com/1188362#c19).
			conn, err := cr.NewConn(ctx, "chrome://newtab/")
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
			location := nodeInfo.Value;

			// Check that test search engine was triggered by the keyword and '{searchTerms}' was replaced in the query by the user's search term.
			keywordPresent := strings.Contains(location, fmt.Sprintf("%s/search?q=%s", testSearchEngine, testSearchTerm))
			if param.enabled != keywordPresent {
				s.Fatalf("Unexpected usage of the search engine keyword: got %t; want %t", keywordPresent, param.enabled)
			}
		})
	}
}
