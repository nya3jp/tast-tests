// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
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
		Func: AppPermissionOsSettings,
		Desc: "Checks the App Notification permission toggle functionality",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func AppPermissionOsSettings(ctx context.Context, s *testing.State) {
	pollOpts := &testing.PollOptions{Timeout: 10 * time.Second}

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

	// Install the "Arc Notification Test" toggle.
	s.Logf("Installing %s", apk)
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatalf("Failed to install %s: %v", apk, err)
	}

	// function to find notification
	findNotification := func() (*ash.Notification, error) {
		ns, err := ash.Notifications(ctx, tconn)
		if err != nil {
			return nil, err
		}
		for _, n := range ns {
			if strings.Contains(n.ID, notificationID) {
				return n, nil
			}
		}
		return nil, errors.New("notification not found")
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

	// Create notification when toggle is OFF
	d := s.FixtValue().(*arc.PreData).UIDevice
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

	// Verify Notification is not shown
	var notif *ash.Notification
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := findNotification()
		if err != nil {
			return err
		}
		notif = n
		return nil
	}, pollOpts); err != nil {
		s.Fatal("Notification was shown: ", err)
	}

	// Click on notification toggle from OFF to ON
	const arcNotificationTitle = "ARC Notification Test"
	if err := appSettings.LeftClick(nodewith.Name(arcNotificationTitle).Role(role.ToggleButton))(ctx); err != nil {
		s.Fatal("Failed to toggle on Arc Notification Test notification permission: ", err)
	}

	// Create notification when toggle is ON
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

	// Verify Notification is shown
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := findNotification()
		if err != nil {
			return err
		}
		notif = n
		return nil
	}, pollOpts); err == nil {
		s.Fatal("Notification was shown: ", err)
	}
}
