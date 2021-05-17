// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

const (
	notificationRemoveID = notificationIDPrefix + "remove_button"

	pkg = "org.chromium.arc.testapp.notification"
	cls = ".NotificationActivity"

	notificationIDPrefix = pkg + ":id/"
	titleID              = notificationIDPrefix + "notification_title"
	textID               = notificationIDPrefix + "notification_text"
	idID                 = notificationIDPrefix + "notification_id"
	sendID               = notificationIDPrefix + "send_button"

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

func init() {
	testing.AddTest(&testing.Test{
		Func: Notification,
		Desc: "Launches a testing APK to generate notification and verifies its state",
		Contacts: []string{
			"edcourtney@chromium.org", // Notification owner.
			"arc-framework+tast@google.com",
			"hidehiko@chromium.org", // Tast port author.
			"cros-arc-te@google.com",
		},
		Attr:         []string{"group:mainline", "group:arc-functional"},
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

func findNotification(ctx context.Context, tconn *chrome.TestConn) (*ash.Notification, error) {
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

// runTest runs the test case, and uses removeNotification func to remove the notifications.
func runTest(ctx context.Context, removeNotification func() error, a *arc.ARC, tconn *chrome.TestConn) error {
	pollOpts := &testing.PollOptions{Timeout: 10 * time.Second}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close(ctx)
	// Create a notification.
	if err := d.Object(ui.ID(titleID)).SetText(ctx, title); err != nil {
		return errors.Wrapf(err, "failed to set text to %s", titleID)
	}
	if err := d.Object(ui.ID(textID)).SetText(ctx, text); err != nil {
		return errors.Wrapf(err, "failed to set text to %s", textID)
	}
	if err := d.Object(ui.ID(idID)).SetText(ctx, msgID); err != nil {
		return errors.Wrapf(err, "failed to set text to %s", idID)
	}
	if err := d.Object(ui.ID(sendID)).Click(ctx); err != nil {
		return errors.Wrapf(err, "failed to click %s button", sendID)
	}

	var notif *ash.Notification
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := findNotification(ctx, tconn)
		if err != nil {
			return err
		}
		notif = n
		return nil
	}, pollOpts); err != nil {
		return errors.Wrap(err, "notification wasn't shown")
	}

	if notif.Title != title {
		return errors.Errorf("unexpected notification title: got %q; want %q", notif.Title, title)
	}
	if notif.Message != text {
		return errors.Errorf("unexpected notification message: got %q; want %q", notif.Message, text)
	}

	// Update the title.
	if err := d.Object(ui.ID(titleID)).SetText(ctx, title2); err != nil {
		return errors.Wrapf(err, "failed to set text to %s", titleID)
	}
	if err := d.Object(ui.ID(sendID)).Click(ctx); err != nil {
		return errors.Wrapf(err, "failed to click %s button", sendID)
	}

	// Wait for that the title is updated.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := findNotification(ctx, tconn)
		if err != nil {
			return err
		}
		if n.Title != title2 {
			return errors.Errorf("unexpected title: got %q; want %q", n.Title, title2)
		}
		return nil
	}, pollOpts); err != nil {
		return errors.Wrap(err, "notification wasn't updated")
	}
	if err := removeNotification(); err != nil {
		return errors.Wrap(err, "failed to remove notification")
	}

	// Wait for that the notification was removed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := findNotification(ctx, tconn); err == nil {
			return errors.New("notification still visible")
		}
		return nil
	}, pollOpts); err != nil {
		return errors.Wrap(err, "notification wasn't removed")
	}
	return nil
}

func Notification(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	a := s.FixtValue().(*arc.PreData).ARC

	const apk = "ArcNotificationTest.apk"
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

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	s.Log("Setup is done, and running the test scenario")

	// Try removing the notification from Android first.
	testing.ContextLog(ctx, "removing notification from android")
	if err := runTest(ctx, func() error {
		d, err := a.NewUIDevice(ctx)
		if err != nil {
			s.Fatal("Failed to initialize UI Automator: ", err)
		}
		defer d.Close(ctx)
		// Remove the notification.
		if err := d.Object(ui.ID(notificationRemoveID)).Click(ctx); err != nil {
			s.Fatalf("Failed to click %s button: %v", notificationRemoveID, err)
		}
		return nil
	}, a, tconn); err != nil {
		s.Fatal("Failed to remove notifications from Android: ", err)
	}

	testing.ContextLog(ctx, "removing notification from chrome - clearALL")
	// Now try removing the notification using Chrome 'clearAll'.
	if err := runTest(ctx, func() error {
		cr := s.FixtValue().(*arc.PreData).Chrome
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}
		// Open Quick Settings to ensure the 'Clear all' button is available.
		if err := quicksettings.Show(ctx, tconn); err != nil {
			s.Fatal("Failed to open Quick Settings: ", err)
		}
		defer quicksettings.Hide(ctx, tconn)

		clearAllParams := chromeui.FindParams{Name: "Clear all", Role: chromeui.RoleTypeStaticText}
		clearAll, err := chromeui.FindWithTimeout(ctx, tconn, clearAllParams, 10*time.Second)
		if err != nil {
			s.Fatal("Failed to find 'Clear all' button: ", err)
		}
		defer clearAll.Release(ctx)
		if err := clearAll.LeftClick(ctx); err != nil {
			s.Fatal("Failed to click 'Clear all' button: ", err)
		}
		return nil
	}, a, tconn); err != nil {
		s.Fatal("Failed to remove notifications from Android: ", err)
	}
}
