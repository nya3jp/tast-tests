// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils provides funcs to cleanup folders in ChromeOS.
package utils

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// DisplayLayout is a pair of internal and external display.Info.
type DisplayLayout struct {
	Internal display.Info
	External display.Info
}

// DisplayInfo returns display.Info by display type.
func (layout *DisplayLayout) DisplayInfo(displayType arc.DisplayType) *display.Info {
	if displayType == arc.InternalDisplay {
		return &layout.Internal
	} else if displayType == arc.ExternalDisplay {
		return &layout.External
	}
	panic("Out of index")
}

// cursorOnDisplay remembers which display the mouse cursor is on.
type cursorOnDisplay struct {
	currentDisp     int
	currentDispType arc.DisplayType
}

// moveTo mouse.Move does not move the cursor out side of the display. To overcome the limitation, this method place a mouse cursor around display edge by mouse.Move, then moves cursor by raw input.MouseEventWriter to cross display boundary.
func (cursor *cursorOnDisplay) moveTo(ctx context.Context, tconn *chrome.TestConn, m *input.MouseEventWriter, dstDisp int, dstDispType arc.DisplayType, layout DisplayLayout) error {
	// Validates display layout
	intBnds := layout.Internal.Bounds
	extBnds := layout.External.Bounds
	if intBnds.Left != 0 || intBnds.Top != 0 || extBnds.Left != intBnds.Width || extBnds.Top != 0 {
		wantIntBnds := coords.NewRect(0, 0, intBnds.Width, intBnds.Height)
		wantExtBnds := coords.NewRect(intBnds.Width, 0, extBnds.Width, extBnds.Height)
		return errors.Errorf("moveTo method assumes external display is placed on the right edge of the default display; got: (intDisp %q extDisp %q), want: (intDisp %q extDisp %q)", intBnds, extBnds, wantIntBnds, wantExtBnds)
	}

	if cursor.currentDisp == dstDisp {
		return nil
	}

	var start coords.Point
	var delta coords.Point
	const coordsMargin = 100
	if cursor.currentDispType == arc.InternalDisplay && dstDispType == arc.ExternalDisplay {
		start = coords.NewPoint(layout.Internal.Bounds.Width-coordsMargin, coordsMargin)
		delta = coords.NewPoint(1, 0)
	} else if cursor.currentDispType == arc.ExternalDisplay && dstDispType == arc.InternalDisplay {
		start = coords.NewPoint(coordsMargin, coordsMargin)
		delta = coords.NewPoint(-1, 0)
	} else {
		return errors.Errorf("unexpected display: current %d, destination %d", cursor.currentDisp, dstDisp)
	}

	if err := mouse.Move(tconn, start, 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the mouse")
	}
	for i := 0; i < coordsMargin*2; i++ {
		if err := m.Move(int32(delta.X), int32(delta.Y)); err != nil {
			return err
		}
		testing.Sleep(ctx, 5*time.Millisecond)
	}
	cursor.currentDisp = dstDisp
	cursor.currentDispType = dstDispType
	return nil
}
