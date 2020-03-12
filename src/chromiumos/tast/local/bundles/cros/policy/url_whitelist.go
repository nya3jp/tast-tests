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
		Func: URLWhitelist,
		Desc: "Behavior of the URLWhitelist policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"vsavu@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func URLWhitelist(ctx context.Context, s *testing.State) {
	// ToDo: Move this function to a common package (apart from the test table), and use it in url_blacklist.go as well.
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, testCase := range []struct {
		name        string          // Name is the subtest name.
		blockedURLs []string        // BlockedURLs is a list of urls expected to be blocked.
		allowedURLs []string        // AllowedURLs is a list of urls expected to be accessible.
		newPolicies []policy.Policy // Policies to update before a test.
	}{
		{
			name:        "unset",
			blockedURLs: []string{},
			allowedURLs: []string{"http://chromium.org"},
			newPolicies: []policy.Policy{
				&policy.URLBlacklist{Stat: policy.StatusUnset},
				&policy.URLWhitelist{Stat: policy.StatusUnset},
			},
		},
		{
			name:        "single",
			blockedURLs: []string{"http://example.org"},
			allowedURLs: []string{"http://chromium.org"},
			newPolicies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"org"}},
				&policy.URLWhitelist{Val: []string{"chromium.org"}},
			},
		},
		{
			name:        "identical",
			blockedURLs: []string{"http://example.org"},
			allowedURLs: []string{"http://chromium.org"},
			newPolicies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"http://chromium.org", "http://example.org"}},
				&policy.URLWhitelist{Val: []string{"http://chromium.org"}},
			},
		},
		{
			name:        "https",
			blockedURLs: []string{"http://chromium.org"},
			allowedURLs: []string{"https://chromium.org"},
			newPolicies: []policy.Policy{
				&policy.URLBlacklist{Val: []string{"chromium.org"}},
				&policy.URLWhitelist{Val: []string{"https://chromium.org"}},
			},
		},
	} {
		s.Run(ctx, testCase.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update URLBlacklist and URLWhitelist policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, testCase.newPolicies); err != nil {
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
