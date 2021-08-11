// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NotificationsBlockedForUrls,
		Desc: "Behavior of NotificationsBlockedForUrls policy: checking if notifications are blocked for a specified url",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"notifications_for_urls_test_page.html"},
	})
}

// NotificationsBlockedForUrls tests the NotificationsBlockedForUrls policy.
func NotificationsBlockedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	includesBlockedURL := []string{"http://www.bing.com", server.URL,
		"https://www.yahoo.com"}
	excludesBlockedURL := []string{"http://www.bing.com",
		"https://www.irs.gov/",
		"https://www.yahoo.com"}

	for _, param := range []struct {
		name                string
		notificationBlocked bool            // notification_blocked determines whether notifications are blocked in the test case or not.
		policies            []policy.Policy // list of policies to be set.
	}{
		{
			name:                "site_blocked_block",
			notificationBlocked: true,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Val: includesBlockedURL},
				&policy.DefaultNotificationsSetting{Val: 1},
			},
		},
		{
			name:                "site_allowed_show",
			notificationBlocked: false,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Val: excludesBlockedURL},
				&policy.DefaultNotificationsSetting{Val: 1},
			},
		},
		{
			name:                "not_set_show",
			notificationBlocked: false,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Stat: policy.StatusUnset},
				&policy.DefaultNotificationsSetting{Val: 1},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the test page.
			conn, err := cr.NewConn(ctx, server.URL+"/notifications_for_urls_test_page.html")
			if err != nil {
				s.Fatal("Failed to connect to the policy page: ", err)
			}
			defer conn.Close()

			var notificationPermission string
			if err := conn.Eval(ctx, `Notification.permission`, &notificationPermission); err != nil {
				s.Fatal("Could not read notification permission: ", err)
			}

			if notificationPermission != "granted" && notificationPermission != "denied" && notificationPermission != "default" {
				s.Fatal("Unable to capture Notification Setting")
			}
			notificationBlocked := notificationPermission == "denied"
			// Check if the notification permission is inline with the expected permission.
			if notificationBlocked != param.notificationBlocked {
				s.Errorf("Unexpected permission for notifications: got %v; want %v", notificationBlocked, param.notificationBlocked)
			}
		})
	}
}
