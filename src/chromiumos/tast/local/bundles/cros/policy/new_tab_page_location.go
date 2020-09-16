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
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS
	exp := "chrome://settings/"

	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{&policy.NewTabPageLocation{Val: exp}}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	conn, err := cr.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	var url string
	if err := conn.Eval(ctx, `document.URL`, &url); err != nil {
		s.Fatal("Could not read URL: ", err)
	}

	if url != exp {
		s.Errorf("New tab navigated to %s, expected %s", url, exp)
	}
}
