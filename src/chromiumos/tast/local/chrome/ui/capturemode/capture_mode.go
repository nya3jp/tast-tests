// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package capturemode contains helper methods to work with Capture Mode.
package capturemode

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// ErrCaptureModeNotFound is returned by TakeAreaScreenshot if capture mode was not
// found in the UI.
//
// For example, capture mode might be not allowed by admin policy.
var ErrCaptureModeNotFound = errors.New("capture mode not found in the UI")

func enterCaptureMode(ctx context.Context, tconn *chrome.TestConn) error {
	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show system tray")
	}

	params := ui.FindParams{Name: "Screen capture", ClassName: "FeaturePodIconButton"}

	if err := ui.StableFindAndClick(ctx, tconn, params, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to find and click capture mode button")
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

	params := ui.FindParams{Name: "Capture"}
	if err := ui.StableFindAndClick(ctx, tconn, params, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		// Return ErrCaptureModeNotFound if capture mode UI does not exist, so caller can handle this case separately.
		if exists, errExist := ui.Exists(ctx, tconn, params); errExist == nil && !exists {
			return ErrCaptureModeNotFound
		}
		return errors.Wrap(err, "failed to find and click capture button")
	}

	return nil
}
