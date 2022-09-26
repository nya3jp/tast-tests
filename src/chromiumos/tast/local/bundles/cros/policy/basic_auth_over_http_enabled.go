// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicAuthOverHTTPEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the BasicAuthOverHttpEnabled policy is properly applied",
		Contacts: []string{
			"jityao@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.BasicAuthOverHttpEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func BasicAuthOverHTTPEnabled(ctx context.Context, s *testing.State) {
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

	// Create a server with basic auth.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Please enter your username and password for this site"`)
		// Always deny access, so that it requires auth and shows the authentication prompt window.
		w.WriteHeader(401)
		w.Write([]byte("Unauthorised.\n"))
	}))
	defer server.Close()

	for _, param := range []struct {
		name       string
		value      *policy.BasicAuthOverHttpEnabled
		wantPrompt bool
	}{
		{
			name:       "enabled",
			value:      &policy.BasicAuthOverHttpEnabled{Val: true},
			wantPrompt: true,
		},
		{
			name:       "disabled",
			value:      &policy.BasicAuthOverHttpEnabled{Val: false},
			wantPrompt: false,
		},
		{
			name:       "unset",
			value:      &policy.BasicAuthOverHttpEnabled{Stat: policy.StatusUnset},
			wantPrompt: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeLacros)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}

			// Navigate to a page which requires authentication.
			if err := conn.Navigate(ctx, server.URL); err != nil {
				s.Fatalf("Failed to navigate to the server URL %q: %v", server.URL, err)
			}
			defer conn.Close()

			// Create a uiauto.Context with default timeout.
			ui := uiauto.New(tconn)

			promptWindow := nodewith.Name("Sign in").Role(role.Window)
			if param.wantPrompt {
				if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(promptWindow.First())(ctx); err != nil {
					s.Fatal("Failed to find the prompt window for authentication: ", err)
				}
			} else {
				if err := ui.EnsureGoneFor(promptWindow, 10*time.Second)(ctx); err != nil {
					s.Fatal("Failed to make sure no authentication prompt window popup: ", err)
				}
			}
		})
	}
}
