// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type popupsSettingTestTable struct {
	name        string          // name is the subtest name.
	browserType browser.Type    // browser type used in the subtest.
	wantTitle   string          // wantTitle is the expected title of the window after test is run with policies applied.
	policies    []policy.Policy // policies is a list of PopupsBlockedForUrls, PopupsAllowedForUrls and DefaultPopupsSetting policies to update before checking popups.
}

// TODO(crbug.com/1125586): investigate using an easier filter like "*" in the allow/deny-listing policies along with DefaultPopupsSetting policy.
const filterPopupsURL = "http://*/popups_for_url_check_index.html"

func init() {
	testing.AddTest(&testing.Test{
		Func:         PopupsForURLCheck,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the behavior of popups on URL allow/deny-listing user policies",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"gabormagda@google.com",
			"alexanderhartl@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Data:         []string{"popups_for_url_check_index.html", "popups_for_url_check_popup.html"},
		Params: []testing.Param{
			{
				Name:    "default",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []popupsSettingTestTable{
					{
						name:        "allowed",
						browserType: browser.TypeAsh,
						wantTitle:   "Popups allowed",
						policies:    []policy.Policy{&policy.DefaultPopupsSetting{Val: 1}}, // 1: Popups are allowed
					},
					{
						name:        "blocked",
						browserType: browser.TypeAsh,
						wantTitle:   "Popups blocked",
						policies:    []policy.Policy{&policy.DefaultPopupsSetting{Val: 2}}, // 2: Popups are blocked
					},
					{
						name:        "unset",
						browserType: browser.TypeAsh,
						wantTitle:   "Popups blocked",
						policies:    []policy.Policy{&policy.DefaultPopupsSetting{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:    "allowlist",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []popupsSettingTestTable{
					{
						name:        "blocklist_unset_default_block",
						browserType: browser.TypeAsh,
						wantTitle:   "Popups allowed",
						policies: []policy.Policy{
							&policy.PopupsBlockedForUrls{Stat: policy.StatusUnset},
							&policy.PopupsAllowedForUrls{Val: []string{filterPopupsURL}},
							&policy.DefaultPopupsSetting{Val: 2}, // 2: Popups are blocked by default
						},
					},
					{
						name:        "blocklist_set_default_block",
						browserType: browser.TypeAsh,
						wantTitle:   "Popups allowed",
						policies: []policy.Policy{
							&policy.PopupsBlockedForUrls{Val: []string{"https://chromium.org", "http://example.org"}},
							&policy.PopupsAllowedForUrls{Val: []string{filterPopupsURL}},
							&policy.DefaultPopupsSetting{Val: 2}, // 2: Popups are blocked by default
						},
					},
				},
			},
			{
				Name:    "blocklist",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []popupsSettingTestTable{
					{
						name:        "allowlist_unset_default_allow",
						browserType: browser.TypeAsh,
						wantTitle:   "Popups blocked",
						policies: []policy.Policy{
							&policy.PopupsBlockedForUrls{Val: []string{filterPopupsURL}},
							&policy.PopupsAllowedForUrls{Stat: policy.StatusUnset},
							&policy.DefaultPopupsSetting{Val: 1}, // 1: Popups are allowed by default
						},
					},
					{
						name:        "allowlist_set_default_allow",
						browserType: browser.TypeAsh,
						wantTitle:   "Popups blocked",
						policies: []policy.Policy{
							&policy.PopupsBlockedForUrls{Val: []string{filterPopupsURL}},
							&policy.PopupsAllowedForUrls{Val: []string{"https://chromium.org", "http://example.org"}},
							&policy.DefaultPopupsSetting{Val: 1}, // 1: Popups are allowed by default
						},
					},
				},
			},
			{
				Name:              "lacros_default",
				Fixture:           fixture.LacrosPolicyLoggedIn,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				Val: []popupsSettingTestTable{
					{
						name:        "allowed",
						browserType: browser.TypeLacros,
						wantTitle:   "Popups allowed",
						policies:    []policy.Policy{&policy.DefaultPopupsSetting{Val: 1}}, // 1: Popups are allowed
					},
					{
						name:        "blocked",
						browserType: browser.TypeLacros,
						wantTitle:   "Popups blocked",
						policies:    []policy.Policy{&policy.DefaultPopupsSetting{Val: 2}}, // 2: Popups are blocked
					},
					{
						name:        "unset",
						browserType: browser.TypeLacros,
						wantTitle:   "Popups blocked",
						policies:    []policy.Policy{&policy.DefaultPopupsSetting{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:              "lacros_allowlist",
				Fixture:           fixture.LacrosPolicyLoggedIn,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				Val: []popupsSettingTestTable{
					{
						name:        "blocklist_unset_default_block",
						browserType: browser.TypeLacros,
						wantTitle:   "Popups allowed",
						policies: []policy.Policy{
							&policy.PopupsBlockedForUrls{Stat: policy.StatusUnset},
							&policy.PopupsAllowedForUrls{Val: []string{filterPopupsURL}},
							&policy.DefaultPopupsSetting{Val: 2}, // 2: Popups are blocked by default
						},
					},
					{
						name:        "blocklist_set_default_block",
						browserType: browser.TypeLacros,
						wantTitle:   "Popups allowed",
						policies: []policy.Policy{
							&policy.PopupsBlockedForUrls{Val: []string{"https://chromium.org", "http://example.org"}},
							&policy.PopupsAllowedForUrls{Val: []string{filterPopupsURL}},
							&policy.DefaultPopupsSetting{Val: 2}, // 2: Popups are blocked by default
						},
					},
				},
			},
			{
				Name:              "lacros_blocklist",
				Fixture:           fixture.LacrosPolicyLoggedIn,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				Val: []popupsSettingTestTable{
					{
						name:        "allowlist_unset_default_allow",
						browserType: browser.TypeLacros,
						wantTitle:   "Popups blocked",
						policies: []policy.Policy{
							&policy.PopupsBlockedForUrls{Val: []string{filterPopupsURL}},
							&policy.PopupsAllowedForUrls{Stat: policy.StatusUnset},
							&policy.DefaultPopupsSetting{Val: 1}, // 1: Popups are allowed by default
						},
					},
					{
						name:        "allowlist_set_default_allow",
						browserType: browser.TypeLacros,
						wantTitle:   "Popups blocked",
						policies: []policy.Policy{
							&policy.PopupsBlockedForUrls{Val: []string{filterPopupsURL}},
							&policy.PopupsAllowedForUrls{Val: []string{"https://chromium.org", "http://example.org"}},
							&policy.DefaultPopupsSetting{Val: 1}, // 1: Popups are allowed by default
						},
					},
				},
			},
		},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DefaultPopupsSetting{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.PopupsAllowedForUrls{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.PopupsBlockedForUrls{}, pci.VerifiedValue),
		},
	})
}

func PopupsForURLCheck(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	tcs, ok := s.Param().([]popupsSettingTestTable)
	if !ok {
		s.Fatal("Failed to convert test cases to the desired type")
	}

	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, tc.browserType)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, server.URL+"/popups_for_url_check_index.html")
			if err != nil {
				s.Fatal("Creating renderer failed: ", err)
			}
			defer conn.Close()

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get TestConn: ", err)
			}

			// Wait until the popup window is opened.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				windows, err := ash.GetAllWindows(ctx, tconn)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get windows"))
				}

				for _, window := range windows {
					if strings.Contains(window.Title, tc.wantTitle) {
						return nil
					}
				}
				return errors.New("failed to find expected window title")
			}, &testing.PollOptions{
				Timeout: 15 * time.Second,
			}); err != nil {
				s.Errorf("Failed to find window title %q", tc.wantTitle)
			}
		})
	}
}
