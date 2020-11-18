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
	"chromiumos/tast/testing"
)

var (
	systemTrayFindParams       = ui.FindParams{ClassName: "SystemTrayContainer"}
	systemTrayButtonFindParams = ui.FindParams{Role: ui.RoleTypeButton, ClassName: "UnifiedSystemTray"}
)

// ToggleSystemTray clicks on the system tray button.
func ToggleSystemTray(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ui.StableFindAndClick(ctx, tconn, systemTrayButtonFindParams, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to stable find and click system tray button")
	}
	return nil
}

// ShowSystemTray clicks on the system tray button to show the system tray buttons if system tray is hidden.
func ShowSystemTray(ctx context.Context, tconn *chrome.TestConn) error {
	if exist, err := ui.Exists(ctx, tconn, systemTrayFindParams); err != nil {
		return errors.Wrap(err, "failed to check whether system tray exists")
	} else if exist {
		// System tray is already visible.
		return nil
	}

	if err := ToggleSystemTray(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle system tray")
	}

	if err := ui.WaitUntilExists(ctx, tconn, systemTrayFindParams, 10*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	return nil
}

// HideSystemTray clicks on the system tray button to hide the system tray buttons if system tray is visible.
func HideSystemTray(ctx context.Context, tconn *chrome.TestConn) error {
	if exist, err := ui.Exists(ctx, tconn, systemTrayFindParams); err != nil {
		return errors.Wrap(err, "failed to check whether system tray exists")
	} else if !exist {
		// System tray is already hidden.
		return nil
	}

	if err := ToggleSystemTray(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle system tray")
	}

	if err := ui.WaitUntilGone(ctx, tconn, systemTrayFindParams, 10*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	return nil
}
