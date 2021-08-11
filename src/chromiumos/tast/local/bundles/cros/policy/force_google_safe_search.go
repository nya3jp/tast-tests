// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ForceGoogleSafeSearch,
		Desc: "Test the behavior of ForceGoogleSafeSearch policy: check if Google safe search is enabled based on the value of the policy",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func ForceGoogleSafeSearch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, param := range []struct {
		name     string
		wantSafe bool
		value    *policy.ForceGoogleSafeSearch
	}{
		{
			name:     "enabled",
			wantSafe: true,
			value:    &policy.ForceGoogleSafeSearch{Val: true},
		},
		{
			name:     "disabled",
			wantSafe: false,
			value:    &policy.ForceGoogleSafeSearch{Val: false},
		},
		{
			name:     "unset",
			wantSafe: false,
			value:    &policy.ForceGoogleSafeSearch{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, "https://www.google.com/search?q=kittens")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var isSafe bool
			if err := conn.Eval(ctx, `new URL(document.URL).searchParams.get("safe") == "active"`, &isSafe); err != nil {
				s.Fatal("Could not read safe search param from URL: ", err)
			}

			if isSafe != param.wantSafe {
				s.Errorf("Unexpected safe search behavior; got %t, want %t", isSafe, param.wantSafe)
			}
		})
	}
}
