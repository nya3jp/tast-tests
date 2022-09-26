// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"io"
	"net"
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
		Func:         AllowCrossOriginAuthPrompt,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the behavior of 3rd part resources on pages whether it shows auth prompt or not",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AllowCrossOriginAuthPrompt{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// htmlPageWithCORSRequest returns html page with specified port.
// Cross-Origin indicates any other origins (domain, scheme, or port) and since the test is using 127.0.0.1,
// we must use localhost here instead, so that the browser could see this link as a different origin.
func htmlPageWithCORSRequest(port int) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">

<head>

  <meta charset="utf-8">
  <title>AllowCrossOriginAuthPrompt</title>
</head>
<body>

  <div id="imagePage">
    <img src='http://localhost:%v' alt='img' id='img_id'>
  </div>
</body>
</html>
`, port)
}

func AllowCrossOriginAuthPrompt(ctx context.Context, s *testing.State) {
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
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Please enter your username and password for this site"`)
		// Always deny access, so that it requires auth and shows the authentication prompt window.
		w.WriteHeader(401)
		w.Write([]byte("Unauthorised.\n"))
	}))
	defer authServer.Close()

	port := authServer.Listener.Addr().(*net.TCPAddr).Port

	// Create a server that will serve html page with a link that require auth (http://localhost:port).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, htmlPageWithCORSRequest(port))
	}))
	defer server.Close()

	for _, param := range []struct {
		name       string
		wantPrompt bool                               // wantPrompt is to check if the page should show an authentication prompt.
		value      *policy.AllowCrossOriginAuthPrompt // value is the value of the policy.
	}{
		{
			name:       "enabled",
			wantPrompt: true,
			value:      &policy.AllowCrossOriginAuthPrompt{Val: true},
		},
		{
			name:       "disabled",
			wantPrompt: false,
			value:      &policy.AllowCrossOriginAuthPrompt{Val: false},
		},
		{
			name:       "unset",
			wantPrompt: false,
			value:      &policy.AllowCrossOriginAuthPrompt{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}

			// Open a page that contains a link which requires authentication.
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
