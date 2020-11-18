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
	"chromiumos/tast/local/coords"
)

// OpenSystemTray clicks on the tray button to show and hide the system tray buttons.
func EnterCaptureMode(ctx context.Context, tconn *chrome.TestConn) error {
	captureModeButton, err := ui.Find(ctx, tconn, ui.FindParams{Attributes: map[string]interface{}{"name": ""}, ClassName: "FeaturePodIconButton"})
	if err != nil {
		return errors.Wrap(err, "failed to find the capture mode button")
	}
	defer captureModeButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, captureModeButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the capture mode button")
	}

	return nil
}

func TakeAreaScreenshot(ctx context.Context, tconn *chrome.TestConn) error {
	if err := mouse.Drag(ctx, tconn, coords.Point{X: 0, Y: 0}, coords.Point{X: 100, Y: 100}, 0*time.Second); err != nil {
		return errors.Wrap(err, "failed to drag mouse")
	}

	captureButton, err := ui.Find(ctx, tconn, ui.FindParams{Name: "Capture"})
	if err != nil {
		return errors.Wrap(err, "failed to find the capture button")
	}
	defer captureButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, captureButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the capture button")
	}

	return nil
}
