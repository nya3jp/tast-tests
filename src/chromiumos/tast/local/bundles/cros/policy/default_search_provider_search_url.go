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
	"chromiumos/tast/errors"
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
		Attr:         []string{"group:mainline"},
		Pre:          pre.User,
	})
}

func DefaultSearchProviderSearchURL(ctx context.Context, s *testing.State) {
	const (
		fakeURL    = "fakeurl" // fakeURL is the fake search engine.
		searchTerm = "abc"     // searchTerm is a value for test search.
	)

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
			addressBarParams := ui.FindParams{
				Role: ui.RoleTypeTextField,
				Name: "Address and search bar",
			}

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
			if err := kb.Type(ctx, searchTerm+"\n"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Find the address bar.
			addressBar, err := ui.FindWithTimeout(ctx, tconn, addressBarParams, 10*time.Second)
			if err != nil {
				s.Fatal("Failed to find the address bar: ", err)
			}
			defer addressBar.Release(ctx)

			var location string
			// Wait for the address bar value change.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				addressBar.Update(ctx)
				if addressBar.Value != "about:blank" && addressBar.Value != searchTerm {
					location = addressBar.Value
					return nil
				}
				return errors.New("failed to wait for address bar value change")
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				s.Fatal("Failed to wait for address bar value change: ", err)
			}

			if !strings.HasPrefix(location, param.wantURL) {
				s.Fatalf("Unexpected search engine used: got %q; want %q", location, param.wantURL)
			}
		})
	}
}
