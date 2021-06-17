// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

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
		Func: DefaultSearchProviderEnabled,
		Desc: "Behavior of DefaultSearchProviderEnabled policy: check if a search provider is being automatically used",
		Contacts: []string{
			"anastasiian@chromium.org",
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DefaultSearchProviderEnabled(ctx context.Context, s *testing.State) {
	const (
		defaultSearchEngine = "google.com" // search engine checked in the test
	)
	addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar")

	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	uiauto := uiauto.New(tconn).WithTimeout(10 * time.Second)

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	for _, param := range []struct {
		name    string                               // name is the subtest name.
		enabled bool                                 // enabled is the expected enabled state of the policy.
		value   *policy.DefaultSearchProviderEnabled // value is the policy value.
	}{
		{
			name:    "true",
			enabled: true,
			value:   &policy.DefaultSearchProviderEnabled{Val: true},
		},
		{
			name:    "false",
			enabled: false,
			value:   &policy.DefaultSearchProviderEnabled{Val: false},
		},
		{
			name:    "unset",
			enabled: true,
			value:   &policy.DefaultSearchProviderEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
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

			// Type something.
			if err := kb.Type(ctx, "abc\n"); err != nil {
				s.Fatal("Failed to write events: ", err)
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

			defaultSearchEngineUsed := strings.Contains(location, defaultSearchEngine)
			if param.enabled != defaultSearchEngineUsed {
				s.Fatalf("Unexpected usage of search engine: got %t; want %t (got %q; want %q)", defaultSearchEngineUsed, param.enabled, location, defaultSearchEngine)
			}
		})
	}
}
