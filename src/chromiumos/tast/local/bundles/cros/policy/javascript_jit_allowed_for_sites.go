// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         JavascriptJITAllowedForSites,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check that the JavascriptJITAllowedForSites policy allows the JIT compiler for the specified URLs",
		Contacts: []string{
			"eariassoto@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
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
		Data: []string{"jit_test.html", "is_jit_enabled.wasm"},
	})
}

func JavascriptJITAllowedForSites(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	url := server.URL + "/jit_test.html"
	defer server.Close()

	for _, tc := range []struct {
		name  string
		value *policy.JavaScriptJitAllowedForSites
		allow bool
	}{
		{
			name:  "set_site_match_pattern",
			value: &policy.JavaScriptJitAllowedForSites{Val: []string{url}},
			allow: true,
		},
		{
			name:  "set_site_unmatch_pattern",
			value: &policy.JavaScriptJitAllowedForSites{Val: []string{"https://my_website.com/*"}},
			allow: false,
		},
		{
			name:  "unset",
			value: &policy.JavaScriptJitAllowedForSites{Stat: policy.StatusUnset},
			allow: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Error("Failed to reset Chrome: ", err)
			}

			// By default the JIT compiler setting is to allow it for all sites.
			// This test will change the JIT default policy to block. The policy
			// under test will have priority for sites that match the URL pattern.
			defaultJitPol := &policy.DefaultJavaScriptJitSetting{Val: 2}
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{defaultJitPol, tc.value}); err != nil {
				s.Error("Failed to serve and verify policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Error("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, url)
			if err != nil {
				s.Error("Failed to connect to Chrome: ", err)
			}
			defer conn.Close()

			jitAllowed := false
			if err := conn.Eval(ctx, "isJitEnabled()", &jitAllowed); err != nil {
				s.Error("Could not evaluate function isJitEnabled: ", err)
			}

			if jitAllowed != tc.allow {
				s.Errorf("Unexpected JIT compiler status: got %v, want %v", jitAllowed, tc.allow)
			}
		})
	}
}
