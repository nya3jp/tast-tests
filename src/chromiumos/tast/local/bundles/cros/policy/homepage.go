// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type homepageSettingTestTable struct {
	name         string          // name is the subtest name.
	browserType  browser.Type    // browser type used in the subtest; must match the fixture.
	wantHomepage bool            // wantHomepage is whether the homepage is expected to be the one set in the HomepageLocation policy.
	policies     []policy.Policy // policies is a list of HomepageLocation and HomepageIsNewTabPage policies to update before checking the homepage.
}

const chromePoliciesURL = "chrome://policy/"

func init() {
	testing.AddTest(&testing.Test{
		Func:         Homepage,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of the HomepageLocation and HomepageIsNewTabPage policies",
		Contacts: []string{
			"snijhara@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:commercial_limited"},
		Params: []testing.Param{
			{
				Name:    "location",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []homepageSettingTestTable{
					{
						name:         "set",
						browserType:  browser.TypeAsh,
						wantHomepage: true,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Val: false},
						},
					},
					{
						name:         "unset",
						browserType:  browser.TypeAsh,
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Stat: policy.StatusUnset},
							&policy.HomepageIsNewTabPage{Val: false},
						},
					},
				},
			},
			{
				Name:              "lacros_location",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []homepageSettingTestTable{
					{
						name:         "set",
						browserType:  browser.TypeLacros,
						wantHomepage: true,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Val: false},
						},
					},
					{
						name:         "unset",
						browserType:  browser.TypeLacros,
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Stat: policy.StatusUnset},
							&policy.HomepageIsNewTabPage{Val: false},
						},
					},
				},
			},
			{
				Name:    "is_new_tab_page",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []homepageSettingTestTable{
					// The test case for HomepageIsNewTabPage{Val: false} is not present here as it is already included in the above group.
					{
						name:         "set_true",
						browserType:  browser.TypeAsh,
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Val: true},
						},
					},
					{
						name:         "unset",
						browserType:  browser.TypeAsh,
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Stat: policy.StatusUnset},
						},
					},
				},
			},
			{
				Name:              "lacros_is_new_tab_page",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []homepageSettingTestTable{
					// The test case for HomepageIsNewTabPage{Val: false} is not present here as it is already included in the above group.
					{
						name:         "set_true",
						browserType:  browser.TypeLacros,
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Val: true},
						},
					},
					{
						name:         "unset",
						browserType:  browser.TypeLacros,
						wantHomepage: false,
						policies: []policy.Policy{
							&policy.HomepageLocation{Val: chromePoliciesURL},
							&policy.HomepageIsNewTabPage{Stat: policy.StatusUnset},
						},
					},
				},
			},
		},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.HomepageLocation{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.HomepageIsNewTabPage{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func Homepage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, tc.browserType)
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, "chrome://version/")
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
