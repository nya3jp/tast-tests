// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NotificationsBlockedForUrls,
		Desc: "Behavior of NotificationsBlockedForUrls policy: checking if notifications are blocked for a specified url",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"notifications_for_urls_test_page.html"},
	})
}

// NotificationsBlockedForUrls tests the NotificationsBlockedForUrls policy.
func NotificationsBlockedForUrls(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	includesBlockedURL := []string{"http://www.bing.com", server.URL,
		"https://www.yahoo.com"}
	excludesBlockedURL := []string{"http://www.bing.com",
		"https://www.irs.gov/",
		"https://www.yahoo.com"}

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name                 string
		notification_blocked bool            // notification_blocked determines whether notifications are blocked in the test case or not.
		policies             []policy.Policy // list of policies to be set.
	}{
		{
			name:                 "site_blocked_block",
			notification_blocked: true,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Val: includesBlockedURL},
				&policy.DefaultNotificationsSetting{Val: 1},
			},
		},
		{
			name:                 "site_allowed_show",
			notification_blocked: false,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Val: excludesBlockedURL},
				&policy.DefaultNotificationsSetting{Val: 1},
			},
		},
		{
			name:                 "not_set_show",
			notification_blocked: false,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Stat: policy.StatusUnset},
				&policy.DefaultNotificationsSetting{Val: 1},
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

			// Open the test page.
			conn, err := cr.NewConn(ctx, server.URL + "/notifications_for_urls_test_page.html")
			if err != nil {
				s.Fatal("Failed to connect to the policy page: ", err)
			}
			defer conn.Close()

			var notification_permission string
			if err := conn.Eval(ctx, `Notification.permission`, &notification_permission); err != nil {
				s.Fatal("Could not read notification permission: ", err)
			}

			if notification_permission != "granted" && notification_permission != "denied" && notification_permission != "default" {
				s.Fatal("Unable to capture Notification Setting.")
			}
			notification_blocked := notification_permission == "denied"
			// Check if the notification permission is inline with the expected permission.
			if notification_blocked != param.notification_blocked {
				s.Errorf("Unexpected permission for notifications: got %v; want %v", notification_blocked, param.notification_blocked)
			}
		})
	}
}
