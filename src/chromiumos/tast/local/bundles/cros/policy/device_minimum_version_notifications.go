// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceMinimumVersionNotifications,
		Desc: "Notifications of DeviceMinimumVersion policy when device has reached auto update expiration",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func DeviceMinimumVersionNotifications(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance to fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
		chrome.ExtraArgs("--aue-reached-for-update-required-test"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_update_required_notification")

	// Create and update DeviceMinimumVersion policy.
	policyValue := policy.DeviceMinimumVersion{
		Val: &policy.DeviceMinimumVersionValue{
			Requirements: []*policy.DeviceMinimumVersionValueRequirements{
				{
					AueWarningPeriod: 2,
					ChromeosVersion:  "99999999",
					WarningPeriod:    1,
				},
			},
		},
	}
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{&policyValue}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// Check update required notification in case of auto update expiration is visible.
	const notificationWaitTime = 10 * time.Second
	const notificationID = "policy.update_required" // Hardcoded in Chrome.
	_, err = ash.WaitForNotification(ctx, tconn, notificationWaitTime, ash.WaitIDContains(notificationID))
	if err != nil {
		s.Error("Failed to find update required notification: ", err)
	}

	// Check update required banner in case of auto update expiration is visible on the Chrome management page.
	conn, err := cr.NewConn(ctx, "chrome://management/")
	if err != nil {
		s.Fatal("Failed to open management page: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExprFailOnErr(ctx, "document.querySelector('management-ui').$$('.eol-section') && !document.querySelector('management-ui').$$('.eol-section[hidden]')"); err != nil {
		s.Error("Failed to verify update required end-of-life banner on management page: ", err)
	}
}
