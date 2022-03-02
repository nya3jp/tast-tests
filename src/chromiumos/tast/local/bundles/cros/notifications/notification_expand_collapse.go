// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NotificationExpandCollapse,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test the expand and collapse behavior of a notification",
		Contacts:     []string{"leandre@chromium.org", "cros-status-area-eng@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		Vars: []string{browserfixt.LacrosDeployedBinary},
	})
}

func NotificationExpandCollapse(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	// Start a Chrome instance with the notification refresh feature.
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, s.Param().(browser.Type), chrome.EnableFeatures("NotificationsRefresh"))
	if err != nil {
		s.Fatalf("Failed to connect to %v browser: %v", bt, err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

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
	popup := nodewith.Role(role.Window).HasClass("ash/message_center/MessagePopup")
	notificationHeader := nodewith.HasClass("NotificationHeaderView")
	expandButton := nodewith.HasClass("AshNotificationExpandButton")
	statusArea := nodewith.HasClass("ash/StatusAreaWidgetDelegate")
	notificationInMessageCenter := nodewith.HasClass("MessageViewContainer")

	takeScreenshot := func(ctx context.Context) error {
		if err := vkb.Accel(ctx, "Ctrl+F5"); err != nil {
			return errors.Wrap(err, "failed to take a screenshot")
		}

		// Verify that the screen capture popup notification is shown after taking the screenshot.
		if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(popup)(ctx); err != nil {
			return errors.Wrap(err, "failed to find popup notification")
		}
		return nil
	}

	clickNotificationAndVerify := func(ctx context.Context) error {
		// Verify that the Files app is opended after clicking the notification.
		if err := ui.LeftClick(popup)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the notification body")
		}
		if err := ash.WaitForApp(ctx, tconn, apps.Files.ID, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for Files app to open")
		}

		// Close Files app at the end so we can reopen.
		if err := apps.Close(ctx, tconn, apps.Files.ID); err != nil {
			return errors.Wrap(err, "failed to close Files app")
		}
		return nil
	}

	clickNotificationInMessageCenterAndVerify := func(ctx context.Context) error {
		// Click the notification in message center and verify opening Files app.
		if err := ui.LeftClick(notificationInMessageCenter)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the notification body")
		}
		if err := ash.WaitForApp(ctx, tconn, apps.Files.ID, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for Files app to open")
		}

		// Close Files app at the end so we can reopen.
		if err := apps.Close(ctx, tconn, apps.Files.ID); err != nil {
			return errors.Wrap(err, "failed to close Files app")
		}
		return nil
	}

	if err := uiauto.Combine(
		"generate notification, test collapsing and clicking the notification",
		takeScreenshot,
		ui.WithTimeout(30*time.Second).WaitUntilExists(notificationHeader), // In expanded notification, header is visible.
		ui.LeftClick(expandButton),
		ui.WithTimeout(30*time.Second).WaitUntilGone(notificationHeader), // In collapsed notification, header is not visible.
		clickNotificationAndVerify,
	)(ctx); err != nil {
		s.Fatal("Failed to generate notification, test collapsing and clicking the notification: ", err)
	}

	if err := uiauto.Combine(
		"generate notification, test clicking the expand button twice and verify",
		takeScreenshot,
		ui.LeftClick(expandButton),
		ui.LeftClick(expandButton),
		ui.WithTimeout(30*time.Second).WaitUntilExists(notificationHeader), // In expanded notification, header is visible.
		clickNotificationAndVerify,
	)(ctx); err != nil {
		s.Fatal("Failed to generate notification, test clicking the expand button twice and verify: ", err)
	}

	if err := uiauto.Combine(
		"generate notification, test expand/collapse in message center",
		takeScreenshot,
		ui.LeftClick(statusArea),
		ui.WithTimeout(30*time.Second).WaitUntilExists(notificationHeader), // In expanded notification, header is visible.
		// Collapse the notification. Then close and reopen message center.
		// Notification should remain collapsed (keeping the old state before closing and reopening).
		ui.LeftClick(expandButton),
		ui.LeftClick(statusArea),
		ui.LeftClick(statusArea),
		ui.WithTimeout(30*time.Second).WaitUntilGone(notificationHeader), // In collapsed notification, header is not visible.
		clickNotificationInMessageCenterAndVerify,
	)(ctx); err != nil {
		s.Fatal("Failed to generate notification, test expand/collapse in message center: ", err)
	}

	// Remove all screenshots at the end.
	if err = screenshot.RemoveScreenshots(); err != nil {
		s.Fatal("Failed to remove screenshots: ", err)
	}
}
