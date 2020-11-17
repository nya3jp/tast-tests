// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
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
		Pre:          pre.User,
	})
}

func DefaultSearchProviderSearchURL(ctx context.Context, s *testing.State) {
	const (
		defaultSearchEngine = "google.com" // defaultSearchEngine is the search engine checked in the test.
		testSearchEngine    = "fakeurl"    // testSearchEngine os the fake search engine.
		testSearchTerm      = "abc"        // testSearchTerm is a value for test search.
	)
	addressBarParams := ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Address and search bar",
	}

	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	for _, param := range []struct {
		name    string                                 // name is the subtest name.
		enabled bool                                   // enabled is the expected enabled state of the policy.
		value   *policy.DefaultSearchProviderSearchURL // value is the policy value.
	}{
		{
			name:    "set",
			enabled: true,
			// The URL should include the string '{searchTerms}', replaced in the query by the user's search terms.
			value: &policy.DefaultSearchProviderSearchURL{Val: fmt.Sprintf("https://%s/search?q={searchTerms}", testSearchEngine)},
		},
		{
			name:    "unset",
			enabled: false,
			value:   &policy.DefaultSearchProviderSearchURL{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			// DefaultSearchProviderSearchURL can specify the URL of the search engine only when DefaultSearchProviderEnabled is on.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{&policy.DefaultSearchProviderEnabled{Val: true}, param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Clear the browser history.
			if err := tconn.Eval(ctx, `tast.promisify(chrome.browsingData.removeHistory({"since": 0}))`, nil); err != nil {
				s.Fatal("Failed to clear browsing history: ", err)
			}

			// Open an empty page.
			conn, err := cr.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Click the address and search bar.
			if err := ui.StableFindAndClick(ctx, tconn, addressBarParams, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Failed to click address bar: ", err)
			}

			// Type something.
			if err := kb.Type(ctx, testSearchTerm+"\n"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Wait for the page to load.
			if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
				s.Fatal("Failed to wait for location change: ", err)
			}

			// Find the address bar.
			addressBar, err := ui.FindWithTimeout(ctx, tconn, addressBarParams, 10*time.Second)
			if err != nil {
				s.Fatal("Failed to find the address bar: ", err)
			}
			defer addressBar.Release(ctx)
			location := addressBar.Value

			// Check that test search engine was used and '{searchTerms}' was replaced in the query by the user's search term.
			testSearchEngineUsed := strings.Contains(location, fmt.Sprintf("%s/search?q=%s", testSearchEngine, testSearchTerm))
			defaultSearchEngineUsed := strings.Contains(location, defaultSearchEngine)
			if param.enabled && !testSearchEngineUsed {
				// Test search engine should be used if the policy is enabled.
				s.Fatalf("Unexpected usage of test search engine: got %t; want %t", testSearchEngineUsed, param.enabled)
			} else if !param.enabled && !defaultSearchEngineUsed {
				// Default search engine should be used if the policy is disabled.
				s.Fatalf("Unexpected usage of default search engine: got %t; want %t", defaultSearchEngineUsed, param.enabled)
			}
		})
	}
}
