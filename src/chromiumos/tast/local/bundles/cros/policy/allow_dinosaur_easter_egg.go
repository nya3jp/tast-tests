// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AllowDinosaurEasterEgg,
		Desc: "Behavior of AllowDinosaurEasterEgg policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: "chromePolicyLoggedInWithAsh",
		}, {
			Name:              "lacros",
			Val:               lacros.ChromeTypeLacros,
			Fixture:           "chromePolicyLoggedInWithLacros",
			ExtraData:         []string{launcher.DataArtifact},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func AllowDinosaurEasterEgg(ctx context.Context, s *testing.State) {
	br := s.FixtValue().(*fixtures.FixtData).Browser
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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
			if err := policyutil.ResetChrome(ctx, fdms, br); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, br, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			conn, err := br.NewConn(ctx, "chrome://dino")
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
