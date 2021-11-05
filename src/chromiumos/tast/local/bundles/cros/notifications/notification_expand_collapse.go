// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NotificationExpandCollapse,
		Desc:         "Test the expand and collapse behavior of a notification",
		Contacts:     []string{"leandre@chromium.org", "cros-status-area-eng@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func NotificationExpandCollapse(ctx context.Context, s *testing.State) {
	// Start a Chrome instance with the notification refresh feature.
	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("NotificationsRefresh"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Take a screenshot to show a notification. Using the virtual keyboard is required since
	// different physical keyboards can require different key combinations to take a screenshot.
	vkb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer vkb.Close()

	ui := uiauto.New(tconn)
	popup := nodewith.Role(role.Window).ClassName("ash/message_center/MessagePopup")
	notificationHeader := nodewith.ClassName("NotificationHeaderView")
	expandButton := nodewith.ClassName("AshNotificationExpandButton")
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")

	takeScreenshot := func() {
		if err := vkb.Accel(ctx, "Ctrl+F5"); err != nil {
			s.Fatal("Failed to take a screenshot: ", err)
		}
	}

	clickExpandButton := func() {
		if err := ui.LeftClick(expandButton)(ctx); err != nil {
			s.Fatal("Failed to click expand/collapse button: ", err)
		}
	}

	verifyNotificationIsExpanded := func() {
		// In expanded notification, header is visible.
		if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(notificationHeader)(ctx); err != nil {
			s.Fatal("Failed to find notification header: ", err)
		}
	}

	verifyNotificationIsCollapsed := func() {
		// In collapsed notification, header is not visible.
		if err := ui.WithTimeout(30 * time.Second).WaitUntilGone(notificationHeader)(ctx); err != nil {
			s.Fatal("Failed to verify that notification header is hidden: ", err)
		}
	}

	clickNotificationAndVerify := func() {
		// Verify that the Files app is opended after clicking the notification.
		if err := ui.LeftClick(popup)(ctx); err != nil {
			s.Fatal("Failed to click the notification body: ", err)
		}
		if err := ash.WaitForApp(ctx, tconn, apps.Files.ID, time.Minute); err != nil {
			s.Fatal("Failed to wait for Files app to open: ", err)
		}

		// Close Files app at the end so we can reopen.
		if err := apps.Close(ctx, tconn, apps.Files.ID); err != nil {
			s.Log("Failed to close Files app: ", err)
		}
	}

	clickStatusArea := func() {
		if err := ui.LeftClick(statusArea)(ctx); err != nil {
			s.Fatal("Failed to click expand/collapse button: ", err)
		}
	}

	// Begin testing, take a screenshot.
	takeScreenshot()

	// Verify that the screen capture popup notification is shown after taking the screenshot.
	if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(popup)(ctx); err != nil {
		s.Fatal("Failed to find popup notification: ", err)
	}

	// The screenshot notification should be expanded.
	verifyNotificationIsExpanded()

	// Click expand/collapse button to collapse the notification. Notification should be collapsed.
	clickExpandButton()
	verifyNotificationIsCollapsed()

	clickNotificationAndVerify()

	// Take another screenshot.
	takeScreenshot()

	// Click the expand button twice to collapse and then expand the notification.
	clickExpandButton()
	clickExpandButton()
	verifyNotificationIsExpanded()

	clickNotificationAndVerify()

	// Take another screenshot.
	takeScreenshot()

	// Click Status Area button to show up message center. Notificaton should be expanded here.
	clickStatusArea()
	verifyNotificationIsExpanded()

	// Collapse the notification. Then close and reopen message center.
	// Notification should remain collapsed (keeping the old state before closing and reopening).
	clickExpandButton()
	clickStatusArea()
	clickStatusArea()
	verifyNotificationIsCollapsed()

	// Click the notification in message center and verify opening Files app.
	if err := ui.LeftClick(nodewith.ClassName("MessageViewContainer"))(ctx); err != nil {
		s.Fatal("Failed to click the notification body: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Files.ID, time.Minute); err != nil {
		s.Fatal("Failed to wait for Files app to open: ", err)
	}
}
