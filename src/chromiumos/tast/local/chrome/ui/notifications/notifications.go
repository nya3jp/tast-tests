// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package notifications is used for controlling the notifications state directly through the UI.
package notifications

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	chromeui "chromiumos/tast/local/chrome/ui"
)

// HideAllNotifications clicks on the tray button to show and hide the system tray button, which should also hide any visible notification.
func HideAllNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	trayButton, err := chromeui.Find(ctx, tconn, chromeui.FindParams{Role: chromeui.RoleTypeButton, ClassName: "UnifiedSystemTray"})
	if err != nil {
		return errors.Wrap(err, "system tray button not found")
	}
	defer trayButton.Release(ctx)

	if err := ash.MouseClick(ctx, tconn, trayButton.Location.CenterPoint(), ash.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	if err := chromeui.WaitUntilExists(ctx, tconn, chromeui.FindParams{ClassName: "SettingBubbleContainer"}, 2*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	if err := ash.MouseClick(ctx, tconn, trayButton.Location.CenterPoint(), ash.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}
	return nil
}
