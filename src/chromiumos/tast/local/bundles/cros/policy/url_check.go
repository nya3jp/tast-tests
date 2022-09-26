// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type blocklistTestTable struct {
	name        string          // name is the subtest name.
	browserType browser.Type    // browser type used in the subtest.
	blockedURLs []string        // blockedURLs is a list of urls expected to be blocked.
	allowedURLs []string        // allowedURLs is a list of urls expected to be accessible.
	policies    []policy.Policy // policies is a list of URLBlocklist, URLAllowlist, URLBlacklist and URLWhitelist policies to update before checking urls.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         URLCheck,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the behavior of URL allow/deny-listing policies",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
			"gabormagda@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{
			{
				Name:    "blocklist",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []blocklistTestTable{
					{
						name:        "single",
						browserType: browser.TypeAsh,
						blockedURLs: []string{"http://example.org/blocked.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Val: []string{"http://example.org/blocked.html"}}},
					},
					{
						name:        "multi",
						browserType: browser.TypeAsh,
						blockedURLs: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Val: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"}}},
					},
					{
						name:        "wildcard",
						browserType: browser.TypeAsh,
						blockedURLs: []string{"http://example.com/blocked1.html", "http://example.com/blocked2.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Val: []string{"example.com"}}},
					},
					{
						name:        "chrome-policy",
						browserType: browser.TypeAsh,
						blockedURLs: []string{"chrome://policy"},
						allowedURLs: []string{"http://google.com", "http://chromium.org", "chrome://about"},
						policies:    []policy.Policy{&policy.URLBlocklist{Val: []string{"chrome://policy"}}},
					},
					{
						name:        "wildcard-chrome",
						browserType: browser.TypeAsh,
						blockedURLs: []string{"chrome://about", "chrome://policy"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Val: []string{"chrome://*"}}},
					},
					{
						name:        "unset",
						browserType: browser.TypeAsh,
						blockedURLs: []string{},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:    "allowlist",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []blocklistTestTable{
					{
						name:        "single",
						browserType: browser.TypeAsh,
						blockedURLs: []string{"http://example.org"},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLBlocklist{Val: []string{"org"}},
							&policy.URLAllowlist{Val: []string{"chromium.org"}},
						},
					},
					{
						name:        "identical",
						browserType: browser.TypeAsh,
						blockedURLs: []string{"http://example.org"},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLBlocklist{Val: []string{"http://chromium.org", "http://example.org"}},
							&policy.URLAllowlist{Val: []string{"http://chromium.org"}},
						},
					},
					{
						name:        "https",
						browserType: browser.TypeAsh,
						blockedURLs: []string{"http://chromium.org"},
						allowedURLs: []string{"https://chromium.org"},
						policies: []policy.Policy{
							&policy.URLBlocklist{Val: []string{"chromium.org"}},
							&policy.URLAllowlist{Val: []string{"https://chromium.org"}},
						},
					},
					{
						name:        "unset",
						browserType: browser.TypeAsh,
						blockedURLs: []string{},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLBlocklist{Stat: policy.StatusUnset},
							&policy.URLAllowlist{Stat: policy.StatusUnset},
						},
					},
				},
			},
			{
				Name:              "lacros_blocklist",
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []blocklistTestTable{
					{
						name:        "single",
						browserType: browser.TypeLacros,
						blockedURLs: []string{"http://example.org/blocked.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Val: []string{"http://example.org/blocked.html"}}},
					},
					{
						name:        "multi",
						browserType: browser.TypeLacros,
						blockedURLs: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Val: []string{"http://example.org/blocked1.html", "http://example.org/blocked2.html"}}},
					},
					{
						name:        "wildcard",
						browserType: browser.TypeLacros,
						blockedURLs: []string{"http://example.com/blocked1.html", "http://example.com/blocked2.html"},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Val: []string{"example.com"}}},
					},
					{
						name:        "unset",
						browserType: browser.TypeLacros,
						blockedURLs: []string{},
						allowedURLs: []string{"http://google.com", "http://chromium.org"},
						policies:    []policy.Policy{&policy.URLBlocklist{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:              "lacros_allowlist",
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []blocklistTestTable{
					{
						name:        "single",
						browserType: browser.TypeLacros,
						blockedURLs: []string{"http://example.org"},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLBlocklist{Val: []string{"org"}},
							&policy.URLAllowlist{Val: []string{"chromium.org"}},
						},
					},
					{
						name:        "identical",
						browserType: browser.TypeLacros,
						blockedURLs: []string{"http://example.org"},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLBlocklist{Val: []string{"http://chromium.org", "http://example.org"}},
							&policy.URLAllowlist{Val: []string{"http://chromium.org"}},
						},
					},
					{
						name:        "https",
						browserType: browser.TypeLacros,
						blockedURLs: []string{"http://chromium.org"},
						allowedURLs: []string{"https://chromium.org"},
						policies: []policy.Policy{
							&policy.URLBlocklist{Val: []string{"chromium.org"}},
							&policy.URLAllowlist{Val: []string{"https://chromium.org"}},
						},
					},
					{
						name:        "unset",
						browserType: browser.TypeLacros,
						blockedURLs: []string{},
						allowedURLs: []string{"http://chromium.org"},
						policies: []policy.Policy{
							&policy.URLBlocklist{Stat: policy.StatusUnset},
							&policy.URLAllowlist{Stat: policy.StatusUnset},
						},
					},
				},
			},
		},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.URLBlocklist{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.URLAllowlist{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func URLCheck(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tcs, ok := s.Param().([]blocklistTestTable)
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
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, tc.browserType)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Run actual test.
			urlBlocked := func(url string) bool {
				conn, err := br.NewConn(ctx, url)
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
		})
	}
}
