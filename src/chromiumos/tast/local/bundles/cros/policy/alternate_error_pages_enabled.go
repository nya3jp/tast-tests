// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AlternateErrorPagesEnabled,
		Desc: "Behavior of the AlternateErrorPagesEnabled policy: check that an alternate set of error pages is shown based on the policy",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func AlternateErrorPagesEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, tc := range []struct {
		name       string
		value      *policy.AlternateErrorPagesEnabled
		suggestion string // The first suggestion item on the error page shown when navigating to a nonexistent page.
	}{
		{
			name:       "true",
			value:      &policy.AlternateErrorPagesEnabled{Val: true},
			suggestion: "If spelling is correct, try running Connectivity Diagnostics.",
		},
		{
			name:       "false",
			value:      &policy.AlternateErrorPagesEnabled{Val: false},
			suggestion: "Checking the connection",
		},
		{
			name:       "unset",
			value:      &policy.AlternateErrorPagesEnabled{Stat: policy.StatusUnset},
			suggestion: "If spelling is correct, try running Connectivity Diagnostics.",
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, "https://bogus")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				var line string
				if err := conn.Eval(ctx, "document.querySelector('#suggestions-list li').innerText", &line); err != nil {
					return testing.PollBreak(errors.Wrap(err, "could not read error page suggestion"))
				}

				if line != tc.suggestion {
					return errors.Errorf("unexpected suggestion on the error page; got %q, want %q", line, tc.suggestion)
				}

				return nil
			}, &testing.PollOptions{
				Timeout: 15 * time.Second,
			}); err != nil {
				s.Error("Failed waiting for the correct error page: ", err)
			}
		})
	}
}
