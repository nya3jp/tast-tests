// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package notification contains utilities to help writing ARC notification tests.
package notification

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
)

const (
	// Testing app information.
	apk = "ArcNotificationTest.apk"
	pkg = "org.chromium.arc.testapp.notification"
	cls = ".NotificationActivity"

	// UI IDs in the app.
	idPrefix = pkg + ":id/"
	titleID  = idPrefix + "notification_title"
	textID   = idPrefix + "notification_text"
	idID     = idPrefix + "notification_id"
	sendID   = idPrefix + "send_button"
)

// ARCNotificationTast holds the resource that needed across ARC notification tast test steps.
type ARCNotificationTast struct {
	arc *arc.ARC
	act *arc.Activity
	d   *ui.Device
}

// NewARCNotificationTast creates an ARCNotificationTast by installing the notification testing app,
// launch the app and initialize the UI Automator that needed for generating notifications.
func NewARCNotificationTast(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, outDir string) (*ARCNotificationTast, error) {
	a, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}

	// Installing the testing app.
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		return nil, errors.Wrapf(err, "failed to install %s", apk)
	}

	// Launching the testing app
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new activity: ")
	}

	if err := act.Start(ctx, tconn); err != nil {
		act.Close()
		return nil, errors.Wrap(err, "failed to start the activity: ")
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		act.Close()
		act.Stop(ctx, tconn)
		return nil, errors.Wrap(err, "failed to initialize UI Automator: ")
	}

	return &ARCNotificationTast{arc: a, act: act, d: d}, nil
}

// CreateOrUpdateTestNotification creates a notification with a custom title, text.
// msgID uniquely identify the notification, so if notification needed to be updated, use the same msgID.
func (t *ARCNotificationTast) CreateOrUpdateTestNotification(ctx context.Context, tconn *chrome.TestConn, title, text, msgID string) error {
	// Create or update notification by interacting with the testing app.
	if err := t.d.Object(ui.ID(titleID)).SetText(ctx, title); err != nil {
		return errors.Wrapf(err, "failed to set title to %s", title)
	}
	if err := t.d.Object(ui.ID(textID)).SetText(ctx, text); err != nil {
		return errors.Wrapf(err, "failed to set text to %s", text)
	}
	if err := t.d.Object(ui.ID(idID)).SetText(ctx, msgID); err != nil {
		return errors.Wrapf(err, "failed to set message ID to %s", msgID)
	}
	if err := t.d.Object(ui.ID(sendID)).Click(ctx); err != nil {
		return errors.Wrapf(err, "failed to click %s button", sendID)
	}

	// Make sure that the notification created successfully.
	ns, err := ash.Notifications(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get notifications")
	}

	// Notification ID on Android is composed of many components.
	// This is the substring to match the generated notification.
	notificationID := "|" + pkg + "|" + msgID + "|"
	for _, n := range ns {
		if strings.Contains(n.ID, notificationID) {
			// Notification is found and created successfully.
			return nil
		}
	}
	return errors.New("generated notification not found")
}

// Close terminates all the resources, return the last error encounter.
func (t *ARCNotificationTast) Close(ctx context.Context, tconn *chrome.TestConn) error {
	err := t.d.Close(ctx)
	err = t.act.Stop(ctx, tconn)
	t.act.Close()
	return err
}
