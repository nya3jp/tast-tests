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
		Func: NewTabPageLocation,
		Desc: "Behavior of the NewTabPageLocation policy",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func NewTabPageLocation(ctx context.Context, s *testing.State) {
	for _, param := range []struct {
		name  string
		value *policy.NewTabPageLocation
	}{
		{
			name:  "settings",
			value: &policy.NewTabPageLocation{Val: "chrome://policy/"},
		},
		{
			name:  "unset",
			value: &policy.NewTabPageLocation{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// If the NewTabPageLocation policy is set, when a new tab is opened,
			// the configured page should be loaded. Otherwise, the new tab page is
			// loaded.
			cr := s.PreValue().(*pre.PreData).Chrome
			fdms := s.PreValue().(*pre.PreData).FakeDMS

			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, "chrome://newtab/")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var url string
			if err := conn.Eval(ctx, `document.URL`, &url); err != nil {
				s.Fatal("Could not read URL: ", err)
			}

			if param.value.Stat != policy.StatusUnset {
				if url != param.value.Val {
					s.Errorf("New tab navigated to %s, expected %s", url, param.value.Val)
				}
			} else {
				// Depending on test flags the new tab page url might be one of the following.
				if url != "chrome://new-tab-page/" && url != "chrome://newtab/" && url != "chrome-search://local-ntp/local-ntp.html" {
					s.Errorf("New tab navigated to %s, expected the new tab page", url)
				}
			}
		})
	}
}
