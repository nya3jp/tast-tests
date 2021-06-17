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
		Func: DefaultSearchProviderSearchURL,
		Desc: "Behavior of DefaultSearchProviderSearchURL policy: check if provided search provider is being used",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DefaultSearchProviderSearchURL(ctx context.Context, s *testing.State) {
	const (
		fakeURL    = "fakeurl" // fakeURL is the fake search engine.
		searchTerm = "abc"     // searchTerm is a value for test search.
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
		name    string                                 // name is the subtest name.
		wantURL string                                 // wantURL is the expected search engine url.
		policy  *policy.DefaultSearchProviderSearchURL // policy is the value of DefaultSearchProviderSearchURL policy.
	}{
		{
			name:    "set",
			wantURL: fmt.Sprintf("%s/search?q=%s", fakeURL, searchTerm),
			// The URL should include the string '{searchTerms}', replaced in the query by the user's search terms.
			policy: &policy.DefaultSearchProviderSearchURL{Val: fmt.Sprintf("https://%s/search?q={searchTerms}", fakeURL)},
		},
		{
			name:    "unset",
			wantURL: "google.com",
			policy:  &policy.DefaultSearchProviderSearchURL{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			// DefaultSearchProviderSearchURL can specify the URL of the search engine only when DefaultSearchProviderEnabled is on.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{&policy.DefaultSearchProviderEnabled{Val: true}, param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Clear the browser history, otherwise the previous search results can interfere with the test.
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

			// Type something.
			if err := kb.Type(ctx, searchTerm+"\n"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Wait for the page to load.
			if err := uiauto.WaitForLocation(addressBarNode)(ctx); err != nil {
				s.Fatal("Failed to wait for location change: ", err)
			}

			nodeInfo, err := uiauto.Info(ctx, addressBarNode)
			if err != nil {
				s.Fatal("Could not get new info for the address bar: ", err)
			}
			location := nodeInfo.Value;
			location = strings.TrimPrefix(location, "https://")
			location = strings.TrimPrefix(location, "www.")
			if !strings.HasPrefix(location, param.wantURL) {
				s.Fatalf("Unexpected search engine used: got %q; want %q", location, param.wantURL)
			}
		})
	}
}
