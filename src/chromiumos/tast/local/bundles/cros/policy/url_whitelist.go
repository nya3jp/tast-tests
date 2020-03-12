// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome"
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

type testCasesSt = struct {
	name           string               // Name is the subtest name.
	blacklistValue *policy.URLBlacklist // BlacklistValue is the policy value of URLBlacklist.
	whitelistValue *policy.URLWhitelist // WhitelistValue is the policy value of URLWhitelist.
	blockedURLs    []string             // BlockedURLs is a list of urls expected to be blocked.
	allowedURLs    []string             // AllowedURLs is a list of urls expected to be accessible.
}

var testCases = []testCasesSt{
	{
		name:           "unset",
		blacklistValue: &policy.URLBlacklist{Stat: policy.StatusUnset},
		whitelistValue: &policy.URLWhitelist{Stat: policy.StatusUnset},
		blockedURLs:    []string{},
		allowedURLs:    []string{"http://google.com", "http://chromium.org"},
	},
	{
		name:           "single",
		blacklistValue: &policy.URLBlacklist{Val: []string{"org"}},
		whitelistValue: &policy.URLWhitelist{Val: []string{"chromium.org"}},
		blockedURLs:    []string{"http://example.org"},
		allowedURLs:    []string{"http://google.com", "http://chromium.org"},
	},
	{
		name:           "indentical",
		blacklistValue: &policy.URLBlacklist{Val: []string{"http://example.org", "http://chromium.org"}},
		whitelistValue: &policy.URLWhitelist{Val: []string{"http://chromium.org"}},
		blockedURLs:    []string{"http://example.org"},
		allowedURLs:    []string{"http://google.com", "http://chromium.org"},
	},
	{
		name:           "https",
		blacklistValue: &policy.URLBlacklist{Val: []string{"chromium.org"}},
		whitelistValue: &policy.URLWhitelist{Val: []string{"https://chromium.org"}},
		blockedURLs:    []string{"http://chromium.org"},
		allowedURLs:    []string{"http://google.com", "https://bugs.chromium.org"},
	},
}

func URLWhitelist(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, tc := range testCases {
		test := wrap(testWhiteList, cr, fdms, tc)
		s.Run(ctx, tc.name, test)
	}
}

func wrap(testFunction func(context.Context, *testing.State, *chrome.Chrome, *fakedms.FakeDMS, testCasesSt), cr *chrome.Chrome, fdms *fakedms.FakeDMS, tc testCasesSt) func(context.Context, *testing.State) {
	return func(ctx context.Context, s *testing.State) {
		testFunction(ctx, s, cr, fdms, tc)
	}
}

func testWhiteList(ctx context.Context, s *testing.State, cr *chrome.Chrome, fdms *fakedms.FakeDMS, tc testCasesSt) {
	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Update URLBlacklist and URLWhitelist policies.
	newPolicies := append([]policy.Policy{tc.blacklistValue}, []policy.Policy{tc.whitelistValue}...)
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, newPolicies); err != nil {
		s.Fatal("Failed to update URLBlacklist policies: ", err)
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
}
