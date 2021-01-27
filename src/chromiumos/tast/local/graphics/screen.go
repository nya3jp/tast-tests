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

// Display is a structure used to restore the orientation of the display by
// RestoreDisplayOrientation().
type Display struct {
	dispInfoID     string
	originalOrient *display.Orientation
}

// RestoreDisplayOrientation restores the orientation of the display.
func (d *Display) RestoreDisplayOrientation(ctx context.Context, tconn *chrome.TestConn) error {
	if d.originalOrient == nil {
		// Skips the restoration as the display wasn't rotated in RotateDisplayToLandscapePrimary().
		return nil
	}

	var rotate display.RotationAngle
	switch d.originalOrient.Angle {
	case 0:
		rotate = display.Rotate0
	case 90:
		rotate = display.Rotate90
	case 180:
		rotate = display.Rotate180
	case 270:
		rotate = display.Rotate270
	default:
		return errors.Errorf("unknown rotation %d", d.originalOrient.Angle)
	}

	if err := display.SetDisplayRotationSync(ctx, tconn, d.dispInfoID, rotate); err != nil {
		return errors.Wrap(err, "failed to rotate display")
	}

	orient, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the display orientation")
	}

	if orient != d.originalOrient {
		return errors.New("failed to restore the display rotation")
	}
	// return nil
	return errors.New("Hey this is error")
}

// RotateDisplayToLandscapePrimary rotates the display to landscape-primary defined
// in https://w3c.github.io/screen-orientation/#screenorientation-interface.
// A caller should defer RestoreDisplayOrientation() of the returned Display to restore the orientation.
func RotateDisplayToLandscapePrimary(ctx context.Context, tconn *chrome.TestConn) (*Display, error) {
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get internal display info (this failure can be ignored if the device doesn't have a display e.g. chromebox)")
		return &Display{dispInfoID: "place-holder-id", originalOrient: nil}, nil
	}

	orient, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the display orientation")
	}

	testing.ContextLogf(ctx, "Original display orientation = %q", orient.Type)
	if orient.Type != display.OrientationLandscapePrimary {
		testing.ContextLog(ctx, "Rotating the display to get to 'landscape-primary'")
		if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, display.Rotate270); err != nil {
			return nil, errors.Wrap(err, "failed to rotate display")
		}
		// Make sure that the rotation worked.
		newOrient, err := display.GetOrientation(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get the display orientation")
		}
		if newOrient.Type != display.OrientationLandscapePrimary {
			return nil, errors.New("the display is not in the expected landscape-primary orientation")
		}

	}
	return &Display{dispInfoID: dispInfo.ID, originalOrient: orient}, nil
}
