// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

type homepageSettingTestTable struct {
	name         string          // name is the subtest name.
	wantHomepage bool            // wantHomepage is whether the homepage is expected to be the one set in the HomepageLocation policy.
	policies     []policy.Policy // policies is a list of HomepageLocation and HomepageIsNewTabPage policies to update before checking the homepage.
}

const chromePoliciesURL = "chrome://policy/"

func init() {
	testing.AddTest(&testing.Test{
		Func: Homepage,
		Desc: "Behavior of the HomepageLocation and HomepageIsNewTabPage policies",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Params: []testing.Param{
			{
				Name: "location",
				Val: []homepageSettingTestTable{
					{
						name:         "set",
						wantHomepage: true,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Val: false},
						},
					},
					{
						name:         "unset",
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Stat: policy.StatusUnset},
							&policy.HomepageIsNewTabPage{Val: false},
						},
					},
				},
			},
			{
				Name: "is_new_tab_page",
				Val: []homepageSettingTestTable{
					// The test case for HomepageIsNewTabPage{Val: false} is not present here as it is already included in the above group.
					{
						name:         "set_true",
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Val: true},
						},
					},
					{
						name:         "unset",
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Stat: policy.StatusUnset},
						},
					},
				},
			},
		},
	})
}

func Homepage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	tcs, ok := s.Param().([]homepageSettingTestTable)
	if !ok {
		s.Fatal("Failed to convert test cases to the desired type")
	}

	// (crbug/1153639): It is suspected that some devices might have a special keyboard which
	// is not able to execute the hotkeys in the test and hence makes it flaky on some boards.
	// So, using virtual keyboard here to fix it.
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	// If the HomepageLocation policy is set and HomepageIsNewTabPage is set to false,
	// when the current page is navigated to the home page, the configured page in the
	// HomepageLocation policy should be loaded. Otherwise, the new tab page is loaded.
	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, "chrome://version/")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Navigate to the home page using the hotkey.
			if err := kb.Accel(ctx, "alt+home"); err != nil {
				s.Fatal("Failed to navigate to homepage using hotkey: ", err)
			}

			// Get the current page URL and verify whether it's according to the policy.
			var url string
			if err := conn.Eval(ctx, `document.URL`, &url); err != nil {
				s.Fatal("Could not read URL: ", err)
			}

			if tc.wantHomepage {
				if url != chromePoliciesURL {
					s.Errorf("New tab navigated to %s, expected %s", url, chromePoliciesURL)
				}
				// Depending on test flags the new tab page url might be one of the following.
			} else if url != "chrome://new-tab-page/" && url != "chrome://newtab/" && url != "chrome-search://local-ntp/local-ntp.html" {
				s.Errorf("New tab navigated to %s, expected the new tab page", url)
			}
		})
	}
}
