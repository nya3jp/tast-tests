// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CookiesAllowedForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check that the CookiesAllowedForUrls policy allows setting cookies only on the given sites",
		Contacts: []string{
			"nikitapodguzov@chromium.org", // Test author
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
		Data: []string{"cookies_test.html"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.CookiesAllowedForUrls{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.DefaultCookiesSetting{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func CookiesAllowedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL

	for _, tc := range []struct {
		name  string
		value *policy.CookiesAllowedForUrls
		allow bool
	}{
		{
			name:  "unset",
			value: &policy.CookiesAllowedForUrls{Stat: policy.StatusUnset},
			allow: false,
		},
		{
			name:  "allow_single",
			value: &policy.CookiesAllowedForUrls{Val: []string{url}},
			allow: true,
		},
		{
			name:  "allow_multiple",
			value: &policy.CookiesAllowedForUrls{Val: []string{"http://google.com", "http://doesnotmatter.com", url}},
			allow: true,
		},
		{
			name:  "block_multiple",
			value: &policy.CookiesAllowedForUrls{Val: []string{"https://testingwebsite.html", "https://somewebsite.com", "http://doesnotmatter.com"}},
			allow: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			// For sites not included in allowlist Chrome falls back to default policy.
			blockCookies := &policy.DefaultCookiesSetting{Val: 2}
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value, blockCookies}); err != nil {
				s.Fatal("Failed to serve and verify policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), url+"/cookies_test.html")
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer conn.Close()

			allowed := false
			if err := conn.Eval(ctx, "cookiesAllowed", &allowed); err != nil {
				s.Fatal("Failed to read cookiesAllowed: ", err)
			}

			if allowed != tc.allow {
				s.Fatalf("Unexpected cookiesAllowed value; got %v, want %v", allowed, tc.allow)
			}
		})
	}
}
