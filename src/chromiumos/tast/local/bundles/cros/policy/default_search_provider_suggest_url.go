// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		Func:         DefaultSearchProviderSuggestURL,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of DefaultSearchProviderSuggestURL policy: check if provided search provider is being used for suggestions",
		Contacts: []string{
			"fabiansommer@chromium.org", // Test author
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
			pci.SearchFlag(&policy.DefaultSearchProviderSearchURL{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.DefaultSearchProviderSuggestURL{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func DefaultSearchProviderSuggestURL(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Start a server that will be used as default search engine.
	var srvCallsMutex sync.Mutex
	var srvCalls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Filter out uninteresting calls.
		if r.RequestURI == "/favicon.ico" {
			return
		}
		// Memorize all other calls.
		srvCallsMutex.Lock()
		defer srvCallsMutex.Unlock()
		srvCalls = append(srvCalls, r.RequestURI)
	}))
	defer srv.Close()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Set up keyboard.
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	for _, param := range []struct {
		name          string                                  // subtest name
		searchPolicy  *policy.DefaultSearchProviderSearchURL  // policy value
		suggestPolicy *policy.DefaultSearchProviderSuggestURL // policy value
		wantCalls     []string                                // calls to the server
	}{
		{
			name:          "set",
			searchPolicy:  &policy.DefaultSearchProviderSearchURL{Val: fmt.Sprintf("%s/search?q={searchTerms}", srv.URL)},
			suggestPolicy: &policy.DefaultSearchProviderSuggestURL{Val: fmt.Sprintf("%s/suggest?q={searchTerms}", srv.URL)},
			wantCalls:     []string{"/suggest?q=", "/suggest?q=v", "/suggest?q=vy", "/suggest?q=vy6", "/suggest?q=vy6y", "/suggest?q=vy6ys", "/search?q=vy6ys"},
		},
		{
			name:          "suggestUnset",
			searchPolicy:  &policy.DefaultSearchProviderSearchURL{Val: fmt.Sprintf("%s/search?q={searchTerms}", srv.URL)},
			suggestPolicy: &policy.DefaultSearchProviderSuggestURL{Stat: policy.StatusUnset},
			wantCalls:     []string{"/search?q=vy6ys"},
		},
		{
			name:          "searchUnset",
			searchPolicy:  &policy.DefaultSearchProviderSearchURL{Stat: policy.StatusUnset},
			suggestPolicy: &policy.DefaultSearchProviderSuggestURL{Val: fmt.Sprintf("%s/suggest?q={searchTerms}", srv.URL)},
			wantCalls:     []string{},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Reserve ten seconds for cleanup.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}
			srvCallsMutex.Lock()
			srvCalls = []string{}
			srvCallsMutex.Unlock()

			// Update policies.
			// DefaultSearchProviderSuggestURL only works when both DefaultSearchProviderEnabled is on and DefaultSearchProviderSearchURL is set.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr,
				[]policy.Policy{&policy.DefaultSearchProviderEnabled{Val: true},
					param.searchPolicy,
					param.suggestPolicy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Open an empty page.
			// Use chrome://newtab to open new tab page (see https://crbug.com/1188362#c19).
			conn, err := br.NewConn(ctx, "chrome://newtab/")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Click the address and search bar.
			ui := uiauto.New(tconn)
			addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar")
			if err := ui.LeftClick(addressBarNode)(ctx); err != nil {
				s.Fatal("Could not find the address bar: ", err)
			}

			// Wait until the server processed the initial request.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if len(param.wantCalls) <= 1 {
					// We don't actually query the server here, so just return.
					return nil
				}
				if len(srvCalls) == 1 {
					return nil
				}
				return errors.Errorf("unexpected number of calls to server: got %d, want 1", len(srvCalls))
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Failed to wait for expected calls to the server: ", err)
			}

			// Type something.
			const searchTerm = "vy6ys"
			for i, c := range searchTerm {
				if err := kb.Type(ctx, string(c)); err != nil {
					s.Fatalf("Failed to type %q: %v", c, err)
				}

				// Wait until the server processed the resulting request.
				if err := testing.Poll(ctx, func(ctx context.Context) error {
					if len(param.wantCalls) <= 1 {
						// We don't actually query the server here, so just return.
						return nil
					}
					// For the i-th character (e.g. for "a" i=0) we expect to see i+2 calls to the server (e.g. one for "" and one for "a").
					if len(srvCalls) == i+2 {
						return nil
					}
					return errors.Errorf("unexpected number of calls to server: got %d, want %d", len(srvCalls), i+2)
				}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
					s.Fatal("Failed to wait for expected calls to the server: ", err)
				}
			}

			// Load the page.
			if err := kb.Type(ctx, "\n"); err != nil {
				s.Fatal("Failed to type newline: ", err)
			}

			// Wait until the page loads by checking the address bar.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				currentNodeInfo, err := ui.Info(ctx, addressBarNode)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get info of the address bar"))
				}
				if currentNodeInfo.Value != searchTerm {
					return nil
				}
				return errors.New("the page did not start loading yet")
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				s.Fatal("Failed to wait for the page to load: ", err)
			}

			// Check server calls.
			srvCallsMutex.Lock()
			result := make([]string, len(srvCalls))
			_ = copy(result, srvCalls)
			srvCallsMutex.Unlock()
			if !reflect.DeepEqual(result, param.wantCalls) {
				s.Fatalf("Unexpected calls to server - called URIs: got %s; want %s", result, param.wantCalls)
			}
		})
	}
}
