// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"strings"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// URLBlWhListTestTable is a struct to store test tables used in URLBlacklist
// and URLWhitelist policy tests.
type URLBlWhListTestTable struct {
	Name        string          // Name is the subtest name.
	BlockedURLs []string        // BlockedURLs is a list of urls expected to be blocked.
	AllowedURLs []string        // AllowedURLs is a list of urls expected to be accessible.
	Policies    []policy.Policy // Policies is a list of URLBlacklist and URLWhitelist policies to update before checking urls.
}

// URLBlackWhitelist tests if URLBlacklist and URLWhitelist policies can be set and
// if the appropriate URLs are blocked or allowed.
func URLBlackWhitelist(ctx context.Context, s *testing.State, testTable []URLBlWhListTestTable) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, testCase := range testTable {
		s.Run(ctx, testCase.Name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update URLBlacklist and URLWhitelist policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, testCase.Policies); err != nil {
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

			for _, allowed := range testCase.AllowedURLs {
				if urlBlocked(allowed) {
					s.Errorf("Expected %q to load", allowed)
				}
			}

			for _, blocked := range testCase.BlockedURLs {
				if !urlBlocked(blocked) {
					s.Errorf("Expected %q to be blocked", blocked)
				}
			}
		})
	}
}
