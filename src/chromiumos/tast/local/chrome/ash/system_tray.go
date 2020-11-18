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

// OpenSystemTray clicks on the tray button to show and hide the system tray buttons.
func OpenSystemTray(ctx context.Context, tconn *chrome.TestConn) error {
	trayButton, err := ui.Find(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, ClassName: "UnifiedSystemTray"})
	if err != nil {
		return errors.Wrap(err, "system tray button not found")
	}
	defer trayButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "SystemTrayContainer"}, 2*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	return nil
}
