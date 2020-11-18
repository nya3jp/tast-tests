// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

var (
	systemTrayFindParams       = ui.FindParams{ClassName: "SystemTrayContainer"}
	systemTrayButtonFindParams = ui.FindParams{Role: ui.RoleTypeButton, ClassName: "UnifiedSystemTray"}
)

// ShowSystemTrayIfHidden clicks on the tray button to show the system tray buttons if system tray is hidden.
func ShowSystemTrayIfHidden(ctx context.Context, tconn *chrome.TestConn) error {
	if exist, err := ui.Exists(ctx, tconn, systemTrayFindParams); err != nil {
		return errors.Wrap(err, "failed to check whether system tray exists")
	} else if exist {
		// System tray is already visible.
		return nil
	}

	trayButton, err := ui.Find(ctx, tconn, systemTrayButtonFindParams)
	if err != nil {
		return errors.Wrap(err, "system tray button not found")
	}
	defer trayButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	if err := ui.WaitUntilExists(ctx, tconn, systemTrayFindParams, 2*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	return nil
}

// HideSystemTrayIfVisible clicks on the tray button to hide the system tray buttons if system tray is visible.
func HideSystemTrayIfVisible(ctx context.Context, tconn *chrome.TestConn) error {
	if exist, err := ui.Exists(ctx, tconn, systemTrayFindParams); err != nil {
		return errors.Wrap(err, "failed to check whether system tray exists")
	} else if !exist {
		// System tray is already hidden.
		return nil
	}

	trayButton, err := ui.Find(ctx, tconn, systemTrayButtonFindParams)
	if err != nil {
		return errors.Wrap(err, "system tray button not found")
	}
	defer trayButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	if err := ui.WaitUntilGone(ctx, tconn, systemTrayFindParams, 2*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	return nil
}
