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
		Func: NotificationsAllowedForUrls,
		Desc: "Behavior of NotificationsAllowedForUrls policy: checking if notifications are allowed for a specified url",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Pre:          pre.User,
		Data:         []string{"notifications_for_urls_test_page.html"},
	})
}

// NotificationsAllowedForUrls tests the NotificationsAllowedForUrls policy.
func NotificationsAllowedForUrls(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	includesAllowedURL := []string{"http://www.bing.com", server.URL,
		"https://www.yahoo.com"}
	excludesAllowedURL := []string{"http://www.bing.com",
		"https://www.irs.gov/",
		"https://www.yahoo.com"}

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name                 string
		notification_allowed bool            // notification_allowed determines whether notifications are allowed in the test case or not.
		policies             []policy.Policy // list of policies to be set.
	}{
		{
			name:                 "site_allowed_show",
			notification_allowed: true,
			policies: []policy.Policy{
				&policy.NotificationsAllowedForUrls{Val: includesAllowedURL},
				&policy.DefaultNotificationsSetting{Val: 2},
			},
		},
		{
			name:                 "site_not_allowed_block",
			notification_allowed: false,
			policies: []policy.Policy{
				&policy.NotificationsAllowedForUrls{Val: excludesAllowedURL},
				&policy.DefaultNotificationsSetting{Val: 2},
			},
		},
		{
			name:                 "not_set_block",
			notification_allowed: false,
			policies: []policy.Policy{
				&policy.NotificationsAllowedForUrls{Stat: policy.StatusUnset},
				&policy.DefaultNotificationsSetting{Val: 2},
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
			conn, err := cr.NewConn(ctx, server.URL+"/notifications_for_urls_test_page.html")
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
			notification_allowed := notification_permission == "granted"
			// Check if the notification permission is inline with the expected permission.
			if notification_allowed != param.notification_allowed {
				s.Errorf("Unexpected permission for notifications: got %v; want %v", notification_allowed, param.notification_allowed)
			}
		})
	}
}
