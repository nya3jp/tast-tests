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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// RunTablet runs window arrangement cuj for tablet. Since windows are always
// maximized in tablet mode, we only test performance for tab dragging and split
// view resizing.
func RunTablet(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context) error {
	const (
		timeout = 10 * time.Second
		slow    = 2 * time.Second
		fast    = time.Second / 2
	)

	// Gets primary display info and interesting drag points.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	splitViewDragPoints := DragPoints{
		info.WorkArea.CenterPoint(),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width/4, info.WorkArea.CenterY()),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width-1, info.WorkArea.CenterY()),
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
		return errors.New("browser windows are not snapped yet")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for browser windows to be snapped correctly")
	}

	// Split view resizing by dragging the divider.
	testing.ContextLog(ctx, "Dragging the divider with two snapped browser windows")
	const dividerDragError = "failed to drag divider slightly left, all the way right, and back to center"
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, slow); err != nil {
		return errors.Wrap(err, dividerDragError)
	}

	// Enter the overview mode.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kw.Close()
	topRow, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the top-row layout")
	}
	if err := kw.AccelAction(topRow.SelectTask)(ctx); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}

	// Split view resizing by dragging the divider.
	testing.ContextLog(ctx, "Dragging the divider between an overview window and a snapped browser window")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, slow); err != nil {
		return errors.Wrap(err, dividerDragError)
	}

	// Close the overview window.
	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the browser window in the overview mode to swipe to close")
	}
	swipeStart := w.OverviewInfo.Bounds.CenterPoint()
	if err := pc.Drag(swipeStart, pc.DragTo(swipeStart.Sub(coords.NewPoint(0, 200)), fast))(ctx); err != nil {
		return errors.Wrap(err, "failed to swipe to close overview window")
	}
	// Wait for the swipe-to-close animation to finish before dragging the divider. This is important
	// because the state at the beginning of the divider drag determines which performance metrics
	// are recorded for the entire drag. If the drag begins before the window has actually closed,
	// the resulting data will be for Ash.SplitViewResize.PresentationTime.TabletMode.WithOverview
	// instead of Ash.SplitViewResize.PresentationTime.TabletMode.SingleWindow.
	if err := ui.WaitForLocation(nodewith.Root())(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}

	// Split view resizing by dragging the divider.
	testing.ContextLog(ctx, "Dragging the divider between an empty overview grid and a snapped browser window")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, slow); err != nil {
		return errors.Wrap(err, dividerDragError)
	}

	return nil
}
