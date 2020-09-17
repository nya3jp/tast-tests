// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
)

// Notification corresponds to the "Notification" defined in
// autotest_private.idl.
type Notification struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
	Progress int    `json:"progress"`
}

// Notifications returns an array of notifications in Chrome.
// tconn must be the connection returned by chrome.TestAPIConn().
//
// Note: it uses autotestPrivate API with misleading name getVisibleNotifications
// under the hood.
func Notifications(ctx context.Context, tconn *chrome.TestConn) ([]*Notification, error) {
	var ret []*Notification
	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.getVisibleNotifications)()`, &ret); err != nil {
		return nil, errors.Wrap(err, "failed to call getVisibleNotifications")
	}
	return ret, nil
}

// HideAllNotifications clicks on the tray button to show and hide the system tray button, which should also hide any visible notification.
func HideAllNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	trayButton, err := ui.Find(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, ClassName: "UnifiedSystemTray"})
	if err != nil {
		return errors.Wrap(err, "system tray button not found")
	}
	defer trayButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "SettingBubbleContainer"}, 2*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}
	return nil
}

// CloseNotifications clicks on the tray button to show the system tray button,
// clicks close button on each notification and clicks on the tray button
// to hide the system tray button.
func CloseNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	trayButton, err := ui.Find(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, ClassName: "UnifiedSystemTray"})
	if err != nil {
		return errors.Wrap(err, "system tray button not found")
	}
	defer trayButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "SettingBubbleContainer"}, 2*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	params := ui.FindParams{
		Name:      "Notification close",
		ClassName: "ImageButton",
		Role:      ui.RoleTypeButton,
	}

	notifications, err := Notifications(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get notifications list")
	}

	for i := 0; i < len(notifications); i++ {
		if err := ui.FindAndClick(ctx, tconn, params, 2*time.Second); err != nil {
			return errors.Wrap(err, "failed to find and click on notification close button")
		}
	}

	if err := ui.WaitUntilGone(ctx, tconn, params, 2*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait until there will be no notifications")
	}

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	return nil
}
