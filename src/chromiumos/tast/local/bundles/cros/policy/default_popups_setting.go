// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultPopupsSetting,
		Desc: "Behavior of DefaultPopupsSetting policy, checking if popups are blocked/allowed after setting the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"default_popups_setting_index.html", "default_popups_setting_popup.html"},
	})
}

func DefaultPopupsSetting(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		name          string
		policy        *policy.DefaultPopupsSetting // policy is the policy we test.
		expectedTitle string
	}{
		{
			name:          "allowed",
			expectedTitle: "Popups allowed",
			policy:        &policy.DefaultPopupsSetting{Val: 1}, // 1: Popups are allowed
		},
		{
			name:          "blocked",
			expectedTitle: "Popups blocked",
			policy:        &policy.DefaultPopupsSetting{Val: 2}, // 2: Popups are blocked
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, server.URL+"/default_popups_setting_index.html")
			if err != nil {
				s.Fatal("Creating renderer failed: ", err)
			}
			defer conn.Close()

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get TestConn: ", err)
			}

			// Wait until the popup window is opened.
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				windows, err := ash.GetAllWindows(ctx, tconn)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get windows"))
				}

				for _, window := range windows {
					if strings.Contains(window.Title, param.expectedTitle) {
						return nil
					}
				}

				return errors.New("failed to find expected window title")
			}, nil); err != nil {
				s.Error("DefaultPopupsPolicy failed to be effective: ", err)
			}
		})
	}
}
