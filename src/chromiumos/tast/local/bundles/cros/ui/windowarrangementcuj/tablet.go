// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package windowarrangementcuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// RunTablet runs window arrangement cuj for tablet. Since windows are always
// maximized in tablet mode, we only test performance for tab dragging and split
// view resizing.
func RunTablet(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context) error {
	const (
		timeout  = 10 * time.Second
		duration = 2 * time.Second
	)

	// Gets primary display info and interesting drag points.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	splitViewDragPoints := []coords.Point{
		info.WorkArea.CenterPoint(),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width-1, info.WorkArea.CenterY()),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width/4, info.WorkArea.CenterY()),
	}
	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

	tabStripButton := nodewith.Role(role.Button).ClassName("WebUITabCounterButton").First()
	if err := pc.Click(tabStripButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click the tab strip button")
	}

	firstTab := nodewith.Role(role.Tab).First()
	firstTabRect, err := ui.Location(ctx, firstTab)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the first tab")
	}

	// Drag the first tab in the tab strip and snap it to the right.
	if err := pc.Drag(firstTabRect.CenterPoint(),
		ui.Sleep(time.Second),
		pc.DragTo(snapRightPoint, 3*time.Second),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag a tab to snap to the right")
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if len(ws) != 2 {
			return errors.Errorf("should be 2 windows, got %v", len(ws))
		}
		if (ws[1].State == ash.WindowStateLeftSnapped && ws[0].State == ash.WindowStateRightSnapped) ||
			(ws[0].State == ash.WindowStateLeftSnapped && ws[1].State == ash.WindowStateRightSnapped) {
			return nil
		}
		return errors.New("windows are not snapped yet")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for windows to be snapped correctly")
	}

	// Split view resizing by dragging the divider.
	testing.ContextLog(ctx, "Dragging the divider")
	if err := pc.Drag(splitViewDragPoints[0],
		pc.DragTo(splitViewDragPoints[1], duration),
		pc.DragTo(splitViewDragPoints[2], duration),
		pc.DragTo(splitViewDragPoints[0], duration),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag divider slightly right, all the way left, and back to center")
	}

	return nil
}
