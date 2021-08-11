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
		Func: NewTabPageLocation,
		Desc: "Behavior of the NewTabPageLocation policy",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func NewTabPageLocation(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, tc := range []struct {
		name  string
		value *policy.NewTabPageLocation
	}{
		{
			name:  "set",
			value: &policy.NewTabPageLocation{Val: "chrome://policy/"},
		},
		{
			name:  "unset",
			value: &policy.NewTabPageLocation{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// If the NewTabPageLocation policy is set, when a new tab is opened,
			// the configured page should be loaded. Otherwise, the new tab page is
			// loaded.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, "chrome://newtab/")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var url string
			if err := conn.Eval(ctx, `document.URL`, &url); err != nil {
				s.Fatal("Could not read URL: ", err)
			}

			if tc.value.Stat != policy.StatusUnset {
				if url != tc.value.Val {
					s.Errorf("New tab navigated to %s, expected %s", url, tc.value.Val)
				}
				// Depending on test flags the new tab page url might be one of the following.
			} else if url != "chrome://new-tab-page/" && url != "chrome://newtab/" && url != "chrome-search://local-ntp/local-ntp.html" {
				s.Errorf("New tab navigated to %s, expected the new tab page", url)
			}
		})
	}
}
