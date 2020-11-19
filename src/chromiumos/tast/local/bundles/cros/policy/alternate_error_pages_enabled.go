// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AlternateErrorPagesEnabled,
		Desc: "Check that the AlternateErrorPagesEnabled policy controls wether to show the built-in set of alternate error pages",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func AlternateErrorPagesEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, tc := range []struct {
		name       string
		value      *policy.AlternateErrorPagesEnabled
		suggestion string //The first suggestion item on the error page shown when navigating to a nonexistent page.
	}{
		{
			name:       "unset",
			value:      &policy.AlternateErrorPagesEnabled{Stat: policy.StatusUnset},
			suggestion: "If spelling is correct, try running Connectivity Diagnostics.",
		},
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

			var line string
			if err := conn.Eval(ctx, "document.querySelector('#suggestions-list li').innerText", &line); err != nil {
				s.Fatal("Could not read error page suggestion: ", err)
			}

			if line != tc.suggestion {
				s.Fatalf("Unexpected suggestion on the error page: got %q, wanted %q", line, tc.suggestion)
			}
		})
	}
}
