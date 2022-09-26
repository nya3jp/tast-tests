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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NewTabPageLocation,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of the NewTabPageLocation policy",
		Contacts: []string{
			"mpolzer@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.NewTabPageLocation{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func NewTabPageLocation(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, "chrome://newtab/")
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
