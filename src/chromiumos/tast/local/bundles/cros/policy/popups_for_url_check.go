// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PopupsForURLCheck,
		Desc: "Checks the behavior of URL popups allow/deny-listing user policies",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"default_popups_setting_index.html", "default_popups_setting_popup.html"},
	})
}

func PopupsForURLCheck(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		name          string
		policies      []policy.Policy // policies is a list of PopupsBlockedForUrls, PopupsAllowedForUrls and DefaultPopupsSetting policies to update before checking popups.
		expectedTitle string
	}{
		{
			name:          "blocked_unset_1",
			expectedTitle: "Popups blocked",
			policies: []policy.Policy{
				&policy.PopupsBlockedForUrls{Val: []string{server.URL + "/default_popups_setting_index.html"}},
				&policy.PopupsAllowedForUrls{Stat: policy.StatusUnset},
				&policy.DefaultPopupsSetting{Val: 1}, // 1: Popups are allowed by default
			},
		},
		{
			name:          "blocked_set_1",
			expectedTitle: "Popups blocked",
			policies: []policy.Policy{
				&policy.PopupsBlockedForUrls{Val: []string{server.URL + "/default_popups_setting_index.html"}},
				&policy.PopupsAllowedForUrls{Val: []string{"https://chromium.org", "http://example.org"}},
				&policy.DefaultPopupsSetting{Val: 1}, // 1: Popups are allowed by default
			},
		},
		{
			name:          "unset_allowed_2",
			expectedTitle: "Popups allowed",
			policies: []policy.Policy{
				&policy.PopupsBlockedForUrls{Stat: policy.StatusUnset},
				&policy.PopupsAllowedForUrls{Val: []string{server.URL + "/default_popups_setting_index.html"}},
				&policy.DefaultPopupsSetting{Val: 2}, // 2: Popups are blocked by default
			},
		},
		{
			name:          "set_allowed_2",
			expectedTitle: "Popups allowed",
			policies: []policy.Policy{
				&policy.PopupsBlockedForUrls{Val: []string{"https://chromium.org", "http://example.org"}},
				&policy.PopupsAllowedForUrls{Val: []string{server.URL + "/default_popups_setting_index.html"}},
				&policy.DefaultPopupsSetting{Val: 2}, // 2: Popups are blocked by default
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
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
			}, &testing.PollOptions{
				Timeout: 15 * time.Second,
			}); err != nil {
				s.Errorf("Failed to verify test %s, expected behavior: %s", param.name, param.expectedTitle)
			}
		})
	}
}
