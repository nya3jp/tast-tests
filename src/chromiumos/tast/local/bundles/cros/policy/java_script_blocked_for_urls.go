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
		Func: JavaScriptBlockedForUrls,
		Desc: "Check that the JavaScriptBlockedForUrls policy blocks execution of JavaScript on the given sites",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"js_test.html"},
	})
}

func JavaScriptBlockedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL

	for _, tc := range []struct {
		name  string
		value *policy.JavaScriptBlockedForUrls
		allow bool
	}{
		{
			name:  "unset",
			value: &policy.JavaScriptBlockedForUrls{Stat: policy.StatusUnset},
			allow: true,
		},
		{
			name:  "block_single",
			value: &policy.JavaScriptBlockedForUrls{Val: []string{url + "/js_test.hmtl"}},
			allow: false,
		},
		{
			name:  "allow_multiple",
			value: &policy.JavaScriptBlockedForUrls{Val: []string{"http://www.bing.com", "https://www.yahoo.com"}},
			allow: true,
		},
		{
			name:  "block_multiple",
			value: &policy.JavaScriptBlockedForUrls{Val: []string{"http://www.bing.com", "https://www.yahoo.com", url}},
			allow: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			// Set the default behavior to allow JavaScript.
			allowJavaScript := &policy.DefaultJavaScriptSetting{Val: 1}
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value, allowJavaScript}); err != nil {
				s.Fatal("Failed to serve and verify policies: ", err)
			}

			conn, err := cr.NewConn(ctx, url+"/js_test.html")
			if err != nil {
				s.Fatal("Failed to connect to Chrome: ", err)
			}
			defer conn.Close()

			allowed := false
			// Evaluating is going to fail if JavaScript is not allowed.
			if err := conn.Eval(ctx, "jsAllowed", &allowed); err != nil && tc.allow {
				s.Fatal("Failed to read jsAllowed although JavaScript should be allowed: ", err)
			} else {
				if allowed != tc.allow {
					s.Fatalf("Unexpected jsAllowed value; got %v, want %v", allowed, tc.allow)
				}
			}
		})
	}
}
