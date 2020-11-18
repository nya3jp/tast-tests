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
	"chromiumos/tast/local/coords"
)

func enterCaptureMode(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ShowSystemTrayIfHidden(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show system tray")
	}

	// TODO(crbug.com/1140597): use non-empty Name once CaptureMode will fully
	// support accessability.
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

// TakeAreaScreenshot opens system tray, enters capture mode, selects some area and takes a screenshot.
func TakeAreaScreenshot(ctx context.Context, tconn *chrome.TestConn) error {
	if err := enterCaptureMode(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to enter capture mode")
	}

	// We need to click outside of previous selected area, otherwise we might
	// resize selected area to an empty rectangle and won't see a capture button.
	if err := mouse.Click(ctx, tconn, coords.Point{X: 200, Y: 200}, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click outside of previously selected area")
	}

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
