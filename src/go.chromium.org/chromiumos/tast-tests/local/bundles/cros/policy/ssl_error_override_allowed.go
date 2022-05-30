// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"time"

	"go.chromium.org/chromiumos/tast-tests/common/fixture"
	"go.chromium.org/chromiumos/tast-tests/common/policy"
	"go.chromium.org/chromiumos/tast-tests/common/policy/fakedms"
	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser/browserfixt"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/policyutil"
	"go.chromium.org/chromiumos/tast/testing"
)

const (
	serverKeyFile     = "ssl_error_override_allowed/server.key"
	serverCertFile    = "ssl_error_override_allowed/server.crt" // self-signed -> untrusted CA -> triggers SSL error
	port              = "8090"
	localhostHostname = "localhost"
	localhostIP       = "127.0.0.1"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SSLErrorOverrideAllowed,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of SSlErrorOverrideAllowed and SSLErrorOverrideAllowedForOrigins policy on both Ash and Lacros browser",
		Contacts: []string{
			"hendrich@chromium.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{serverKeyFile, serverCertFile},
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

	// Run http server that triggers SSL error when accessed.
	server := &http.Server{Addr: localhostHostname + ":" + port}
	go func() {
		s.Log("Server has shutdown: ", server.ListenAndServeTLS(s.DataPath(serverCertFile), s.DataPath(serverKeyFile)))
	}()
	defer server.Shutdown(cleanupCtx)

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
			name: "true",
			policies: []policy.Policy{
				&policy.SSLErrorOverrideAllowed{Val: true},
				&policy.SSLErrorOverrideAllowedForOrigins{Stat: policy.StatusUnset},
			},
			expectOverrideAllowedHostname: true,
			expectOverrideAllowedIP:       true,
		},
		{
			name: "false",
			policies: []policy.Policy{
				&policy.SSLErrorOverrideAllowed{Val: false},
				&policy.SSLErrorOverrideAllowedForOrigins{Stat: policy.StatusUnset},
			},
			expectOverrideAllowedHostname: false,
			expectOverrideAllowedIP:       false,
		},
		{
			name: "for_origin",
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

			// Open browser.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Open test API.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get test API connections: ", err)
			}
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Run test for hostname and IP.
			if err := expectOverrideAllowedForURL(ctx, br, tconn, localhostHostname, param.expectOverrideAllowedHostname); err != nil {
				s.Fatal("Failed with hostname: ", err)
			}
			if err := expectOverrideAllowedForURL(ctx, br, tconn, localhostIP, param.expectOverrideAllowedIP); err != nil {
				s.Fatal("Failed with IP: ", err)
			}
		})
	}

}

func expectOverrideAllowedForURL(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn, siteName string, expectOverrideAllowed bool) error {
	// Open browser window.
	conn, err := br.NewConn(ctx, "https://"+siteName+":"+port)
	if err != nil {
		return errors.Wrap(err, "failed to open browser page")
	}
	defer conn.Close()

	// Click "Advanced" button (on SSL error page).
	ui := uiauto.New(tconn)
	advancedButton := nodewith.Name("Advanced").Role(role.Button)
	if err := uiauto.Combine("click advanced",
		ui.WaitUntilExists(advancedButton),
		ui.FocusAndWait(advancedButton),
		ui.LeftClick(advancedButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click advanced button")
	}

	// Check if "proceed" link is visible or not.
	proceedLink := nodewith.Name("Proceed to " + siteName + " (unsafe)").Role(role.Link)
	if expectOverrideAllowed {
		if err := ui.WaitUntilExists(proceedLink)(ctx); err != nil {
			return errors.Wrap(err, "proceed link not visible")
		}
	} else {
		if err := ui.EnsureGoneFor(proceedLink, 2*time.Second)(ctx); err != nil {
			return errors.Wrap(err, "proceed link is visible")
		}
	}

	return nil
}
