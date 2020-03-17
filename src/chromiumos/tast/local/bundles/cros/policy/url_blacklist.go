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

func init() {
	testing.AddTest(&testing.Test{
		Func: URLBlacklist,
		Desc: "Behavior of the URLBlacklist policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
			"gabormagda@google.com",
			"enterprise-policy-support-rotation@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Pre:          pre.User,
		Params: []testing.Param{
			{
				Name:      "blacklist",
				Val:       "blacklist",
				ExtraAttr: []string{"informational"},
			},
			{
				Name:      "whitelist",
				Val:       "whitelist",
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func URLBlacklist(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, testCase := range []struct {
		name        string          // name is the subtest name.
		param       string          // param is compared to test Param to decide whethet the subtest should be run or skipped.
		blockedURLs []string        // blockedURLs is a list of urls expected to be blocked.
		allowedURLs []string        // allowedURLs is a list of urls expected to be accessible.
		policies    []policy.Policy // policies is a list of URLBlacklist and URLWhitelist policies to update before checking urls.
	}{
		{
			name:        "single_blacklist",
			param:       "blacklist",
			blockedURLs: []string{"http://example.org/blocked.html"},
			allowedURLs: []string{"http://google.com", "http://chromium.org"},
			policies:    []policy.Policy{&policy.URLBlacklist{Val: []string{"http://example.org/blocked.html"}}},
		},
		{
			name:        "multi_blacklist",
			param:       "blacklist",
			blockedURLs: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"},
			allowedURLs: []string{"http://google.com", "http://chromium.org"},
			policies:    []policy.Policy{&policy.URLBlacklist{Val: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"}}},
		},
		{
			name:        "wildcard_blacklist",
			param:       "blacklist",
			blockedURLs: []string{"http://example.com/blocked1.html", "http://example.com/blocked2.html"},
			allowedURLs: []string{"http://google.com", "http://chromium.org"},
			policies:    []policy.Policy{&policy.URLBlacklist{Val: []string{"example.com"}}},
		},
		{
			name:        "unset_blacklist",
			param:       "blacklist",
			blockedURLs: []string{},
			allowedURLs: []string{"http://google.com", "http://chromium.org"},
			policies:    []policy.Policy{&policy.URLBlacklist{Stat: policy.StatusUnset}},
		},
		{
			name:        "single_whitelist",
			param:       "whitelist",
			blockedURLs: []string{"http://example.org"},
			allowedURLs: []string{"http://chromium.org"},
			policies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"org"}},
				&policy.URLWhitelist{Val: []string{"chromium.org"}},
			},
		},
		{
			name:        "identical_whitelist",
			param:       "whitelist",
			blockedURLs: []string{"http://example.org"},
			allowedURLs: []string{"http://chromium.org"},
			policies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"http://chromium.org", "http://example.org"}},
				&policy.URLWhitelist{Val: []string{"http://chromium.org"}},
			},
		},
		{
			name:        "https_whitelist",
			param:       "whitelist",
			blockedURLs: []string{"http://chromium.org"},
			allowedURLs: []string{"https://chromium.org"},
			policies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"chromium.org"}},
				&policy.URLWhitelist{Val: []string{"https://chromium.org"}},
			},
		},
		{
			name:        "unset_whitelist",
			param:       "whitelist",
			blockedURLs: []string{},
			allowedURLs: []string{"http://chromium.org"},
			policies: []policy.Policy{
				&policy.URLBlacklist{Stat: policy.StatusUnset},
				&policy.URLWhitelist{Stat: policy.StatusUnset},
			},
		},
	} {
		if testCase.param != s.Param() {
			continue
		}

		s.Run(ctx, testCase.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, testCase.policies); err != nil {
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

			for _, allowed := range testCase.allowedURLs {
				if urlBlocked(allowed) {
					s.Errorf("Expected %q to load", allowed)
				}
			}

			for _, blocked := range testCase.blockedURLs {
				if !urlBlocked(blocked) {
					s.Errorf("Expected %q to be blocked", blocked)
				}
			}
		})
	}
}
