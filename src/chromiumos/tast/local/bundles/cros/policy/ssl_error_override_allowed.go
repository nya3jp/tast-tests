// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"chromiumos/tast/common/fixture"
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

const (
	serverKeyFile     = "ssl_error_override_allowed/server.key"
	serverCertFile    = "ssl_error_override_allowed/server.crt" // self-signed -> untrusted CA
	port              = "8090"
	localhostHostname = "localhost"
	localhostIP       = "127.0.0.1"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SSLErrorOverrideAllowed,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Behavior of SSlErrorOverrideAllowed policy on both Ash and Lacros browser",
		Contacts: []string{
			"hendrich@chromium.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{serverKeyFile, serverCertFile},
		//Fixture:      fixture.ChromePolicyLoggedIn,
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

func SSLErrorOverrideAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	go runServer(cleanupCtx, s)

	for _, param := range []struct {
		name                          string
		policies                      []policy.Policy
		expectOverrideAllowedHostname bool
		expectOverrideAllowedIP       bool
	}{
		{
			name: "unset",
			policies: []policy.Policy{
				&policy.SSLErrorOverrideAllowed{Stat: policy.StatusUnset},
				&policy.SSLErrorOverrideAllowedForOrigins{Stat: policy.StatusUnset},
			},
			expectOverrideAllowedHostname: true,
			expectOverrideAllowedIP:       true,
		},
		{
			name: "SSLErrorOverrideAllowed=true",
			policies: []policy.Policy{
				&policy.SSLErrorOverrideAllowed{Val: true},
				&policy.SSLErrorOverrideAllowedForOrigins{Stat: policy.StatusUnset},
			},
			expectOverrideAllowedHostname: true,
			expectOverrideAllowedIP:       true,
		},
		{
			name: "SSLErrorOverrideAllowed=false",
			policies: []policy.Policy{
				&policy.SSLErrorOverrideAllowed{Val: false},
				&policy.SSLErrorOverrideAllowedForOrigins{Stat: policy.StatusUnset},
			},
			expectOverrideAllowedHostname: false,
			expectOverrideAllowedIP:       false,
		},
		{
			name: "SSLErrorOverrideAllowed=false + SSLErrorOverrideAllowedForOrigins=[https://localhost]",
			policies: []policy.Policy{
				&policy.SSLErrorOverrideAllowed{Val: false},
				&policy.SSLErrorOverrideAllowedForOrigins{Val: []string{"https://" + localhostHostname}},
			},
			expectOverrideAllowedHostname: true,
			expectOverrideAllowedIP:       false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open browser
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(),
				s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get test API connections: ", err)
			}
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			expectOverrideAllowedForURL(ctx, s, br, tconn, localhostHostname, param.expectOverrideAllowedHostname)
			expectOverrideAllowedForURL(ctx, s, br, tconn, localhostIP, param.expectOverrideAllowedIP)
		})
	}

}

func runServer(ctx context.Context, s *testing.State) {
	http.HandleFunc("/", serverResponse)
	server := &http.Server{Addr: localhostHostname + ":" + port}
	log.Fatal("ListenAndServeTLS", server.ListenAndServeTLS(s.DataPath(serverCertFile), s.DataPath(serverKeyFile)))
	defer server.Shutdown(ctx)
}

func serverResponse(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hellow world\n")
}

func expectOverrideAllowedForURL(ctx context.Context, s *testing.State, br *browser.Browser, tconn *chrome.TestConn, siteName string, expectOverrideAllowed bool) {
	// Open browser window with `url`
	conn, err := br.NewConn(ctx, "https://"+siteName+":"+port)
	if err != nil {
		s.Fatal("Failed to open browser page: ", err)
	}
	defer conn.Close()

	// Click "Advanced" button
	ui := uiauto.New(tconn)
	advancedButton := nodewith.Name("Advanced").Role(role.Button)
	if err := uiauto.Combine("click advanced",
		ui.WaitUntilExists(advancedButton),
		ui.FocusAndWait(advancedButton),
		ui.LeftClick(advancedButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click advanced button: ", err)
	}

	// Check if "proceed" link is visible or not
	proceedLink := nodewith.Name("Proceed to " + siteName + " (unsafe)").Role(role.Link)
	if expectOverrideAllowed {
		if err := ui.WaitUntilExists(proceedLink)(ctx); err != nil {
			s.Error("Proceed link not visible: ", err)
		}
	} else {
		if err := ui.EnsureGoneFor(proceedLink, 2*time.Second)(ctx); err != nil {
			s.Error("Proceed link is visible: ", err)
		}
	}
}
