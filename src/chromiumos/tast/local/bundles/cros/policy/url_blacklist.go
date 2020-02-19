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
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func URLBlacklist(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.URLBlacklist
		// blockedURLs is a list of urls expected to be blocked
		blockedURLs []string
		// allowedURLs is a list of urls expected to be accessible
		allowedURLs []string
	}{
		{
			name:        "unset",
			value:       &policy.URLBlacklist{Stat: policy.StatusUnset},
			blockedURLs: []string{},
			allowedURLs: []string{"http://google.com", "http://chromium.org"},
		},
		{
			name:        "single",
			value:       &policy.URLBlacklist{Val: []string{"http://example.org/blocked.html"}},
			blockedURLs: []string{"http://example.org/blocked.html"},
			allowedURLs: []string{"http://google.com", "http://chromium.org"},
		},
		{
			name:        "multi",
			value:       &policy.URLBlacklist{Val: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"}},
			blockedURLs: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"},
			allowedURLs: []string{"http://google.com", "http://chromium.org"},
		},
		{
			name:        "wildcard",
			value:       &policy.URLBlacklist{Val: []string{"example.com"}},
			blockedURLs: []string{"http://example.com/blocked1.html", "http://example.com/blocked2.html"},
			allowedURLs: []string{"http://google.com", "http://chromium.org"},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
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

			for _, allowed := range param.allowedURLs {
				if urlBlocked(allowed) {
					s.Errorf("Expected %q to load", allowed)
				}
			}

			for _, blocked := range param.blockedURLs {
				if !urlBlocked(blocked) {
					s.Errorf("Expected %q to be blocked", blocked)
				}
			}
		})
	}
}
