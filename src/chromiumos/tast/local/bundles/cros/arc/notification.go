// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Notification,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launches a testing APK to generate notification and verifies its state",
		Contacts: []string{
			"yhanada@chromium.org",
			"arc-framework+tast@google.com",
			"hidehiko@chromium.org", // Tast port author.
			"cros-arc-te@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:arc-functional"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:arc-functional"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "refresh",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "refresh_vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func Notification(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

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
	pollOpts := &testing.PollOptions{Timeout: 10 * time.Second}

	s.Logf("Installing %s", apk)
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatalf("Failed to install %s: %v", apk, err)
	}

	s.Log("Launching app")
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	s.Log("Setup is done, and running the test scenario")

	// Create a notification.
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

	var notif *ash.Notification
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := findNotification()
		if err != nil {
			return err
		}
		notif = n
		return nil
	}, pollOpts); err != nil {
		s.Fatal("Notification wasn't shown: ", err)
	}

	if notif.Title != title {
		s.Fatalf("Unexpected notification title: got %q; want %q", notif.Title, title)
	}
	if notif.Message != text {
		s.Fatalf("Unexpected notification message: got %q; want %q", notif.Message, text)
	}

	// Update the title.
	if err := d.Object(ui.ID(titleID)).SetText(ctx, title2); err != nil {
		s.Fatalf("Failed to set text to %s: %v", titleID, err)
	}
	if err := d.Object(ui.ID(sendID)).Click(ctx); err != nil {
		s.Fatalf("Failed to click %s button: %v", sendID, err)
	}

	// Wait for that the title is updated.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := findNotification()
		if err != nil {
			return err
		}
		if n.Title != title2 {
			return errors.Errorf("unexpected title: got %q; want %q", n.Title, title2)
		}
		return nil
	}, pollOpts); err != nil {
		s.Fatal("Notification wasn't updated: ", err)
	}

	// Remove the notification.
	if err := d.Object(ui.ID(removeID)).Click(ctx); err != nil {
		s.Fatalf("Failed to click %s button: %v", removeID, err)
	}

	// Wait for that the notification was removed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := findNotification(); err == nil {
			return errors.New("notification still visible")
		}
		return nil
	}, pollOpts); err != nil {
		s.Fatal("Notification wasn't removed: ", err)
	}
}
