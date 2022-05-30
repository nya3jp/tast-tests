// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package capturemode contains helper methods to work with Capture Mode.
package capturemode

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/mouse"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/quicksettings"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/coords"
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

	ui := uiauto.New(tconn)
	if err := ui.WithTimeout(10 * time.Second).LeftClick(nodewith.Name("Screen capture").ClassName("FeaturePodIconButton"))(ctx); err != nil {
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
	if err := uiauto.Combine("click and drag",
		mouse.Click(tconn, coords.Point{X: 200, Y: 200}, mouse.LeftButton),
		mouse.Drag(tconn, coords.Point{X: 0, Y: 0}, coords.Point{X: 100, Y: 100}, 0*time.Second),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click outside of previously selected area and drag mouse")
	}

	ui := uiauto.New(tconn)
	captureMode := nodewith.Name("Capture").Role(role.Button)
	if err := ui.WithTimeout(10 * time.Second).LeftClick(captureMode)(ctx); err != nil {
		// Return ErrCaptureModeNotFound if capture mode UI does not exist, so caller can handle this case separately.
		if strings.Contains(err.Error(), nodewith.ErrNotFound) {
			return ErrCaptureModeNotFound
		}
		return errors.Wrap(err, "failed to find and click capture button")
	}

	return nil
}
