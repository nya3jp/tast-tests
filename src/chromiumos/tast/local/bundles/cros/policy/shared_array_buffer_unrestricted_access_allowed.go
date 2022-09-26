// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SharedArrayBufferUnrestrictedAccessAllowed,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if SharedArrayBuffer is available in non-cross-origin-isolated contexts depending on the value of this policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.SharedArrayBufferUnrestrictedAccessAllowed{}, pci.VerifiedFunctionalityJS),
		},
	})
}

// SharedArrayBufferUnrestrictedAccessAllowed tests the SharedArrayBufferUnrestrictedAccessAllowed policy.
func SharedArrayBufferUnrestrictedAccessAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "text/plain")
			isolate := r.URL.Query().Get("isolate") == "true"
			if isolate {
				w.Header().Add("Cross-Origin-Embedder-Policy", "require-corp")
				w.Header().Add("Cross-Origin-Opener-Policy", "same-origin")
			}
			fmt.Fprintf(w, "SharedArrayBufferUnrestrictedAccessAllowed test page, isolated: %t", isolate)
		}))
	defer server.Close()

	nonIsolatedURL := fmt.Sprintf("%s/", server.URL)
	isolatedURL := fmt.Sprintf("%s/?isolate=true", server.URL)

	for _, param := range []struct {
		name                            string
		wantAvailableOnNonIsolatedPages bool
		policy                          *policy.SharedArrayBufferUnrestrictedAccessAllowed
	}{
		{
			name:                            "allowed",
			wantAvailableOnNonIsolatedPages: true,
			policy:                          &policy.SharedArrayBufferUnrestrictedAccessAllowed{Val: true},
		},
		{
			name:                            "disallowed",
			wantAvailableOnNonIsolatedPages: false,
			policy:                          &policy.SharedArrayBufferUnrestrictedAccessAllowed{Val: false},
		},
		{
			name:                            "unset",
			wantAvailableOnNonIsolatedPages: false,
			policy:                          &policy.SharedArrayBufferUnrestrictedAccessAllowed{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, browser.TypeLacros, nonIsolatedURL)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer conn.Close()
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Check availability of SharedArrayBuffer on a non-isolated page.
			availableOnNonIsolatedPages := true
			if err := conn.Eval(ctx, "new SharedArrayBuffer()", nil); err != nil {
				availableOnNonIsolatedPages = false
				if param.wantAvailableOnNonIsolatedPages {
					s.Error("Failed to create a new SharedArrayBuffer: ", err)
				}
			}
			if availableOnNonIsolatedPages != param.wantAvailableOnNonIsolatedPages {
				s.Errorf("Unexpected availability of SharedArrayBuffer: got %t; want %t", availableOnNonIsolatedPages, param.wantAvailableOnNonIsolatedPages)
			}

			// Check availability of SharedArrayBuffer on an isolated page.
			if err := conn.Navigate(ctx, isolatedURL); err != nil {
				s.Fatal("Failed to navigate to isolated url: ", err)
			}
			if err := conn.Eval(ctx, "new SharedArrayBuffer()", nil); err != nil {
				s.Error("SharedArrayBuffer is not available on isolated page: ", err)
			}
		})
	}
}
