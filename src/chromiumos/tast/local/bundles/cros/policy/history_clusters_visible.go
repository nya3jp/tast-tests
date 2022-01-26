// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HistoryClustersVisible,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Behavior of HistoryClustersVisible policy",
		Contacts: []string{
			"rodmartin@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.LacrosPolicyLoggedIn,
	})
}

func HistoryClustersVisible(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			conn, err := cr.NewConn(ctx, "chrome://history/journeys")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var isVisibled bool
			if err := conn.Eval(ctx, `document.querySelector("#history-app").shadowRoot.querySelector("#tabs") !== null && document.querySelector("#history-app").shadowRoot.querySelector("#tabs > cr-tabs").shadowRoot.querySelector("div.tab.selected").outerText === 'Journeys'`, &isVisibled); err != nil {
				s.Fatal("Could not read from chrome://history/journeys page: ", err)
			}

			expectedVisibled := param.value.Stat == policy.StatusUnset || param.value.Val

			if isVisibled != expectedVisibled {
				s.Errorf("Unexpected visibility behavior: got %t; want %t for policy", isVisibled, expectedVisibled)
			}
		})
	}
}
