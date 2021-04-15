// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AutoplayAllowed,
		Desc: "Checking if autoplay is allowed on websites or nor, depending on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"autoplay_allowed.html", "audio.mp3"},
	})
}

// AutoplayAllowed tests the AutoplayAllowed policy.
func AutoplayAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name        string
		wantPlaying bool
		policy      *policy.AutoplayAllowed
	}{
		{
			name:        "deny",
			wantPlaying: false,
			policy:      &policy.AutoplayAllowed{Val: false},
		},
		{
			name:        "allow",
			wantPlaying: true,
			policy:      &policy.AutoplayAllowed{Val: true},
		},
		{
			name:        "unset",
			wantPlaying: false,
			policy:      &policy.AutoplayAllowed{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open a website.
			conn, err := cr.NewConn(ctx, server.URL+"/autoplay_allowed.html")
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			// Give the media a second to load so we can determine if it
			// started playing or not.
			testing.Sleep(ctx, time.Second)

			// Check if audio is playing.
			var playing bool
			if err := conn.Eval(ctx, "isMediaPlaying()", &playing); err != nil {
				s.Fatal("Failed to request playing state: ", err)
			}

			// Check if the media is playing.
			if playing != param.wantPlaying {
				s.Errorf("Unexpected media playing state: got %t; want %t", playing, param.wantPlaying)
			}
		})
	}
}
