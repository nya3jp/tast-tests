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
		Data:         []string{"autoplay_allowed.html", "media_tetris.mp3"},
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

	defaultPopupsSettingPolicy := &policy.DefaultPopupsSetting{Val: 1}

	for _, param := range []struct {
		name        string
		wantPlaying bool // wantPlaying states whether access to the microphone is allowed.
		policies    []policy.Policy
	}{
		{
			name:        "deny",
			wantPlaying: false,
			policies: []policy.Policy{
				&policy.AutoplayAllowed{Val: false},
				defaultPopupsSettingPolicy,
			},
		},
		{
			name:        "unset",
			wantPlaying: false,
			policies: []policy.Policy{
				&policy.AutoplayAllowed{Stat: policy.StatusUnset},
				defaultPopupsSettingPolicy,
			},
		},
		{
			name:        "allow",
			wantPlaying: true,
			policies: []policy.Policy{
				&policy.AutoplayAllowed{Val: true},
				defaultPopupsSettingPolicy,
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open a website.
			conn, err := cr.NewConn(ctx, server.URL+"/autoplay_allowed.html")
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			// Give the media a second to load so we can determine if it started playing or not.
			testing.Sleep(ctx, time.Second)

			// Check if audio is playing.
			var playing bool // ec is used to store the error_code.
			if err := conn.Eval(ctx, "isVideoPlaying()", &playing); err != nil {
				s.Fatal("Failed to request playing state: ", err)
			}

			// Check if we got an error while requesting the current position.
			if playing != param.wantPlaying {
				s.Errorf("Unexpected audio playing state: got %t; want %t", playing, param.wantPlaying)
			}
		})
	}
}
