// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppPermissionOsSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks the App Notification permission toggle functionality",
		Contacts: []string{
			"cros-status-area-eng@google.com",
			"newcomer@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "arcvm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func AppPermissionOsSettings(ctx context.Context, s *testing.State) {
	const uiTimeout = 10 * time.Second

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)

	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	a := s.FixtValue().(*arc.PreData).ARC

	const (
		apk = "ArcNotificationTest.apk"
		pkg = "org.chromium.arc.testapp.notification"
		cls = ".NotificationActivity"

		// UI IDs in the app.
		idPrefix = pkg + ":id/"
		titleID  = idPrefix + "notification_title"
		textID   = idPrefix + "notification_text"
		idID     = idPrefix + "notification_id"
		sendID   = idPrefix + "send_button"
		removeID = idPrefix + "remove_button"

		// Testing data.
		title  = "title!"
		title2 = "new title!"
		text   = "hi from Tast"
		msgID  = "12345"

		// Notification ID on Android is composed of many components.
		// This is the substring to match the notification generated
		// earlier.
		notificationID = "|" + pkg + "|" + msgID + "|"
	)

	// Install the "Arc Notification Test" app.
	s.Logf("Installing %s", apk)
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatalf("Failed to install %s: %v", apk, err)
	}

	// Launch App
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	// Launch Notification Subpage
	appNotificationPageHeading := nodewith.NameStartingWith("Notifications").Role(role.Heading).Ancestor(ossettings.WindowFinder)
	appSettings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "app-notifications", uiauto.New(tconn).Exists(appNotificationPageHeading))
	if err != nil {
		s.Fatal("Failed to launch OS Settings")
	}
	d := s.FixtValue().(*arc.PreData).UIDevice

	createNotification := func() {
		if err := d.Object(ui.ID(titleID)).SetText(ctx, title); err != nil {
			s.Fatalf("Failed to set text to %s: %v", titleID, err)
		}
		if err := d.Object(ui.ID(textID)).SetText(ctx, text); err != nil {
			s.Fatalf("Failed to set text to %s: %v", textID, err)
		}
		if err := d.Object(ui.ID(idID)).SetText(ctx, msgID); err != nil {
			s.Fatalf("Failed to set text to %s: %v", idID, err)
		}
		if err := d.Object(ui.ID(sendID)).Click(ctx); err != nil {
			s.Fatalf("Failed to click %s button: %v", sendID, err)
		}
	}

	// Create notification when toggle is ON
	createNotification()

	// Verify Notification is shown
	if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(title)); err != nil {
		s.Fatalf("Failed waiting for %v: %v", title, err)
	}

	// Click on notification toggle from ON to OFF
	const arcNotificationTitle = "ARC Notification Test"
	if err := appSettings.LeftClick(nodewith.Name(arcNotificationTitle).Role(role.ToggleButton))(ctx); err != nil {
		s.Fatal("Failed to toggle on Arc Notification Test notification permission: ", err)
	}

	// Create notification when toggle is OFF
	createNotification()

	// Verify Notification is not shown
	if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(title)); err == nil { // If NO error (notification shown unexpectedly)
		s.Fatal("Notification appeared but it was not supposed to: ", title)
	}
}
