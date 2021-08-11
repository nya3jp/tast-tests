// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CookiesAllowedForUrls,
		Desc: "Check that the CookiesAllowedForUrls policy allows setting cookies only on the given sites",
		Contacts: []string{
			"nikitapodguzov@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"cookies_test.html"},
	})
}

func CookiesAllowedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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

			conn, err := cr.NewConn(ctx, url+"/cookies_test.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
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
