// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/policy"
	"chromiumos/tast/local/policy/fakedms"
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
	})
}

func URLBlacklist(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Set up Chrome Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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
			// Close windows.
			windows, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get windows: ", err)
			}

			for _, window := range windows {
				if err := window.CloseWindow(ctx, tconn); err != nil {
					s.Fatal("Failed to close window: ", err)
				}
			}

			// Create a policy blob and have the FakeDMS serve it.
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicies([]policy.Policy{param.value})
			if err := fdms.WritePolicyBlob(pb); err != nil {
				s.Fatal("Failed to write policies to FakeDMS: ", err)
			}

			// Refresh policies.
			if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)();`, nil); err != nil {
				s.Fatal("Failed to refresh policies: ", err)
			}

			urlBlocked := func(url string) bool {
				conn, err := cr.NewConn(ctx, url)
				if err != nil {
					s.Fatal("Failed to connect to chrome: ", err)
				}
				defer conn.Close()

				var message string
				if err := conn.Eval(ctx, `document.getElementById("main-message").innerText`, &message); err != nil {
					return false // Missing #main-message
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
