// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

// RotateDisplayToLandscapePrimary rotates the display to landscape-primary defined
// in https://w3c.github.io/screen-orientation/#screenorientation-interface.
func RotateDisplayToLandscapePrimary(ctx context.Context, tconn *chrome.TestConn) error {
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	orient, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the display orientation")
	}

	testing.ContextLogf(ctx, "Original display orientation = %q", orient.Type)
	if orient.Type != display.OrientationLandscapePrimary {
		testing.ContextLog(ctx, "Rotating the display to get to 'landscape-primary'")
		if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, display.Rotate270); err != nil {
			return errors.Wrap(err, "failed to rotate display")
		}
		// Make sure that the rotation worked.
		orient, err = display.GetOrientation(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get the display orientation")
		}
		if orient.Type != display.OrientationLandscapePrimary {
			return errors.New("the display is not in the expected landscape-primary orientation")
		}
	}
	return nil
}
