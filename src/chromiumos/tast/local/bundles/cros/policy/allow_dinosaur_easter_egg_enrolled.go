// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AllowDinosaurEasterEggEnrolled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of AllowDinosaurEasterEgg policy on an enrolled device",
		Contacts: []string{
			"vsavu@google.com", // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AllowDinosaurEasterEgg{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func AllowDinosaurEasterEggEnrolled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.AllowDinosaurEasterEgg
	}{
		{
			name:  "true",
			value: &policy.AllowDinosaurEasterEgg{Val: true},
		},
		{
			name:  "false",
			value: &policy.AllowDinosaurEasterEgg{Val: false},
		},
		{
			name:  "unset",
			value: &policy.AllowDinosaurEasterEgg{Stat: policy.StatusUnset},
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
			conn, err := cr.NewConn(ctx, "chrome://dino")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var isBlocked bool
			if err := conn.Eval(ctx, `document.querySelector('* #main-frame-error div.snackbar') === null`, &isBlocked); err != nil {
				s.Fatal("Could not read from dino page: ", err)
			}

			expectedBlocked := param.value.Stat != policy.StatusUnset && param.value.Val

			if isBlocked != expectedBlocked {
				s.Errorf("Unexpected blocked behavior: got %t; want %t", isBlocked, expectedBlocked)
			}
		})
	}
}
