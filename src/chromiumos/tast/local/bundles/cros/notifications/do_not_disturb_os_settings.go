// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DoNotDisturbOSSettings,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks the Do Not Disturb toggle in the OS Settings Notifications subpage",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func DoNotDisturbOSSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// Launch Notification Subpage.
	appNotificationPageHeading := nodewith.NameStartingWith("Notifications").Role(role.Heading).Ancestor(ossettings.WindowFinder)
	appSettings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "app-notifications", ui.Exists(appNotificationPageHeading))
	if err != nil {
		s.Fatal("Failed to launch OS Settings")
	}

	// Toggle ON the DND Toggle.
	const dndTitle = "Do not disturb"
	if err := appSettings.LeftClick(nodewith.Name(dndTitle).Role(role.ToggleButton))(ctx); err != nil {
		s.Fatal("Failed to toggle on DND: ", err)
	}

	// Confirm that Quick Settings panel also reflects its 'DND' toggle to be on.
	if dndEnabled, err := quicksettings.SettingEnabled(ctx, tconn, quicksettings.SettingPodDoNotDisturb); err != nil {
		s.Fatal("Failed to check if Quick Settings Do Not Disturb toggle is ON: ", err)
	} else if !dndEnabled {
		s.Error("Do Not Disturb toggle is OFF when it should be ON")
	}

	const notificationTitle = "notificationTitle"
	const waitForNotificationTimeout = 30 * time.Second

	// Confirm that notification doesn't show when DND is toggled on.
	if _, err := browser.CreateTestNotification(ctx, tconn, browser.NotificationTypeBasic, notificationTitle, "SHOULD NOT SHOW"); err != nil {
		s.Fatal("Failed to create test notification")
	}
	if _, err := ash.WaitForNotification(ctx, tconn, waitForNotificationTimeout, ash.WaitTitle(notificationTitle)); err != nil {
		s.Fatalf("Failed waiting for %v: %v", notificationTitle, err)
	}
	notification := nodewith.Role(role.Window).ClassName("ash/message_center/MessagePopup")
	if err := ui.EnsureGoneFor(notification, 15*time.Second)(ctx); err != nil {
		s.Fatal("Notification was not suppressed")
	}

	// Toggle OFF the DND Toggle
	if err := appSettings.LeftClick(nodewith.Name(dndTitle).Role(role.ToggleButton))(ctx); err != nil {
		s.Fatal("Failed to toggle off DND: ", err)
	}

	// Confirm that notification shows when DND is toggled off.
	if _, err := browser.CreateTestNotification(ctx, tconn, browser.NotificationTypeBasic, notificationTitle, "SHOULD SHOW"); err != nil {
		s.Fatal("Failed to create test notification")
	}
	if _, err := ash.WaitForNotification(ctx, tconn, waitForNotificationTimeout, ash.WaitTitle(notificationTitle)); err != nil {
		s.Fatalf("Failed waiting for %v: %v", notificationTitle, err)
	}
	if err := ui.WaitUntilExists(notification)(ctx); err != nil {
		s.Fatal("Failed to find notification popup: ", err)
	}

	// Confirm that Quick Settings panel also reflects its 'DND' toggle to be off.
	if dndEnabled, err := quicksettings.SettingEnabled(ctx, tconn, quicksettings.SettingPodDoNotDisturb); err != nil {
		s.Fatal("Failed to check if Quick Settings Do Not Disturb toggle is OFF: ", err)
	} else if dndEnabled {
		s.Error("Do Not Disturb toggle is ON when it should be OFF")
	}
}
