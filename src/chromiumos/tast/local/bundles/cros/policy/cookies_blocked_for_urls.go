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
		Func: CookiesBlockedForUrls,
		Desc: "Check that the CookiesBlockedForUrls policy blocks setting cookies on the given sites",
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

func CookiesBlockedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL

	for _, tc := range []struct {
		name  string
		value *policy.CookiesBlockedForUrls
		allow bool
	}{
		{
			name:  "unset",
			value: &policy.CookiesBlockedForUrls{Stat: policy.StatusUnset},
			allow: true,
		},
		{
			name:  "block_single",
			value: &policy.CookiesBlockedForUrls{Val: []string{url}},
			allow: false,
		},
		{
			name:  "block_multiple",
			value: &policy.CookiesBlockedForUrls{Val: []string{"http://google.com", "http://doesnotmatter.com", url}},
			allow: false,
		},
		{
			name:  "allow_multiple",
			value: &policy.CookiesBlockedForUrls{Val: []string{"https://testingwebsite.html", "https://somewebsite.com", "http://doesnotmatter.com"}},
			allow: true,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			// Set the default behavior to allow cookies.
			allowCookies := &policy.DefaultCookiesSetting{Val: 1}
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value, allowCookies}); err != nil {
				s.Fatal("Failed to serve and verify policies: ", err)
			}

			conn, err := cr.NewConn(ctx, url+"/cookies_test.html")
			if err != nil {
				s.Fatal("Failed to connect to Chrome: ", err)
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
