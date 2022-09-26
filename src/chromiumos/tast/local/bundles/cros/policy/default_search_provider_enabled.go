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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultSearchProviderEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of DefaultSearchProviderEnabled policy: check if a search provider is being automatically used",
		Contacts: []string{
			"anastasiian@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DefaultSearchProviderEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func DefaultSearchProviderEnabled(ctx context.Context, s *testing.State) {
	const (
		defaultSearchEngine = "google.com" // search engine checked in the test
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

			// Type something.
			if err := kb.Type(ctx, "vy6ys\n"); err != nil {
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
			location := nodeInfo.Value

			defaultSearchEngineUsed := strings.Contains(location, defaultSearchEngine)
			if param.enabled != defaultSearchEngineUsed {
				s.Fatalf("Unexpected usage of search engine: got %t; want %t (got %q; want %q)", defaultSearchEngineUsed, param.enabled, location, defaultSearchEngine)
			}
		})
	}
}
