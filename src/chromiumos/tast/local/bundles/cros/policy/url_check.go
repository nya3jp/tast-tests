// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type denylistTestTable struct {
	name        string          // name is the subtest name.
	blockedURLs []string        // blockedURLs is a list of urls expected to be blocked.
	allowedURLs []string        // allowedURLs is a list of urls expected to be accessible.
	policies    []policy.Policy // policies is a list of URLDenylist and URLAllowlist policies to update before checking urls.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: URLCheck,
		Desc: "Checks the behavior of URL allow/deny-listing policies",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
			"gabormagda@google.com",
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Pre:          pre.User,
		Params: []testing.Param{
			{
				Name: "denylist",
				Val: []denylistTestTable{
					{
						name:        "single",
						blockedURLs: []string{"http://example.org/blocked.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLDenylist{Val: []string{"http://example.org/blocked.html"}}},
					},
					{
						name:        "multi",
						blockedURLs: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLDenylist{Val: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"}}},
					},
					{
						name:        "wildcard",
						blockedURLs: []string{"http://example.com/blocked1.html", "http://example.com/blocked2.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLDenylist{Val: []string{"example.com"}}},
					},
					{
						name:        "unset",
						blockedURLs: []string{},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLDenylist{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name: "allowlist",
				Val: []denylistTestTable{
					{
						name:        "single",
						blockedURLs: []string{"http://example.org"},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLDenylist{Val: []string{"org"}},
							&policy.URLAllowlist{Val: []string{"chromium.org"}},
						},
					},
					{
						name:        "identical",
						blockedURLs: []string{"http://example.org"},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLDenylist{Val: []string{"http://chromium.org", "http://example.org"}},
							&policy.URLAllowlist{Val: []string{"http://chromium.org"}},
						},
					},
					{
						name:        "https",
						blockedURLs: []string{"http://chromium.org"},
						allowedURLs: []string{"https://chromium.org"},
						policies: []policy.Policy{
							&policy.URLDenylist{Val: []string{"chromium.org"}},
							&policy.URLAllowlist{Val: []string{"https://chromium.org"}},
						},
					},
					{
						name:        "unset",
						blockedURLs: []string{},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLDenylist{Stat: policy.StatusUnset},
							&policy.URLAllowlist{Stat: policy.StatusUnset},
						},
					},
				},
			},
		},
	})
}

func URLCheck(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	tcs, ok := s.Param().([]denylistTestTable)
	if !ok {
		s.Fatal("Failed to convert test cases to the desired type")
	}

	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			urlBlocked := func(url string) bool {
				conn, err := cr.NewConn(ctx, url)
				if err != nil {
					s.Fatal("Failed to connect to chrome: ", err)
				}
				defer conn.Close()

				var message string
				if err := conn.Eval(ctx, `document.getElementById("main-message").innerText`, &message); err != nil {
					return false // Missing #main-message.
				}

				return strings.Contains(message, "ERR_BLOCKED_BY_ADMINISTRATOR")
			}

			for _, allowed := range tc.allowedURLs {
				if urlBlocked(allowed) {
					s.Errorf("Expected %q to load", allowed)
				}
			}

			for _, blocked := range tc.blockedURLs {
				if !urlBlocked(blocked) {
					s.Errorf("Expected %q to be blocked", blocked)
				}
			}
		})
	}
}
