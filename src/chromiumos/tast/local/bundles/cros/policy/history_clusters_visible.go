// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HistoryClustersVisible,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of HistoryClustersVisible policy",
		Contacts: []string{
			"rodmartin@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedInFeatureJourneys,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedInFeatureJourneys,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.HistoryClustersVisible{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func HistoryClustersVisible(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.HistoryClustersVisible
	}{
		{
			name:  "true",
			value: &policy.HistoryClustersVisible{Val: true},
		},
		{
			name:  "false",
			value: &policy.HistoryClustersVisible{Val: false},
		}, {
			name:  "unset",
			value: &policy.HistoryClustersVisible{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := br.NewConn(ctx, "chrome://history/journeys")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Create a uiauto.Context with default timeout.
			ui := uiauto.New(tconn)

			journeys := nodewith.Name("Journeys").Role(role.Tab)
			var isVisible = true
			if err := ui.WithTimeout(2 * time.Second).WaitUntilExists(journeys.First())(ctx); err != nil {
				if !param.value.Val {
					// The Journeys tab shouldn't exist because policy is set to false.
					isVisible = false
				} else {
					// The Journeys tab should exist because policy is set to true or it's unset.
					s.Fatal("Failed to find the prompt window for authentication: ", err)
				}
			}

			expectedVisibility := param.value.Stat == policy.StatusUnset || param.value.Val

			if isVisible != expectedVisibility {
				s.Errorf("Unexpected visibility behavior: got %t; want %t for policy", isVisible, expectedVisibility)
			}
		})
	}
}
