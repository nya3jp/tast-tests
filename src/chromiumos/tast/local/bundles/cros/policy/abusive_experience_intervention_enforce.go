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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

var errNotPlaying = errors.New("media is not playing")

func init() {
	testing.AddTest(&testing.Test{
		Func: AbusiveExperienceInterventionEnforce,
		Desc: "Checking if it is possible to open a link from an abusive experience site, depending on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"abusive_experience1.html", "abusive_experience2.html"},
	})
}

// AbusiveExperienceInterventionEnforce tests the AbusiveExperienceInterventionEnforce policy.
func AbusiveExperienceInterventionEnforce(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		name        string
		wantPlaying bool
		policy      *policy.AbusiveExperienceInterventionEnforce
	}{
		{
			name:         "enabled",
			wantOpenLink: false,
			policy:       &policy.AbusiveExperienceInterventionEnforce{Val: true},
		},
		{
			name:         "disabled",
			wantOpenLink: true,
			policy:       &policy.AbusiveExperienceInterventionEnforce{Val: false},
		},
		{
			name:         "unset",
			wantOpenLink: false,
			policy:       &policy.AbusiveExperienceInterventionEnforce{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fakeDMS, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the website with the media.
			conn, err := cr.NewConn(ctx, server.URL+"/abusive_experience1.html")
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			var openedLink bool
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Check if the the link was opened.
				if err := conn.Eval(ctx, "isMediaPlaying()", &playing); err != nil {
					testing.PollBreak(err)
				}
				if !playing {
					return errNotPlaying
				}
				return nil
			}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: time.Second}); err != nil {
				if !errors.Is(err, errNotPlaying) {
					s.Fatal("Failed to request playing state: ", err)
				}
			}

			if openedLink != param.wantOpenLink {
				s.Errorf("Unexpected opened link state: got %t; want %t", openedLink, param.wantPlwantOpenLinkaying)
			}
		})
	}
}
