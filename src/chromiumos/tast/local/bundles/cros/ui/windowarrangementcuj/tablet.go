// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package windowarrangementcuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// exerciseSplitViewResize assumes two snapped windows and then does the following:
// 1. Drag the divider.
// 2. Enter overview.
// 3. Drag the divider.
// 4. Close the overview window.
// 5. Drag the divider.
func exerciseSplitViewResize(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context, splitViewDragPoints DragPoints, enterOverview action.Action) error {
	const (
		slow = 2 * time.Second
		fast = time.Second / 2
	)

	// 1. Drag the divider.
	testing.ContextLog(ctx, "Dragging the divider between two snapped windows")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, slow); err != nil {
		return errors.Wrap(err, "failed to drag divider between two snapped windows")
	}

	// 2. Enter overview.
	if err := enterOverview(ctx); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}

	// 3. Drag the divider.
	testing.ContextLog(ctx, "Dragging the divider between an overview window and a snapped window")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, slow); err != nil {
		return errors.Wrap(err, "failed to drag divider between overview window and snapped window")
	}

	// 4. Close the first overview window.
	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the window in the overview mode to swipe to close")
	}
	swipeStart := w.OverviewInfo.Bounds.CenterPoint()
	if err := pc.Drag(swipeStart, pc.DragTo(swipeStart.Sub(coords.NewPoint(0, 200)), fast))(ctx); err != nil {
		return errors.Wrap(err, "failed to swipe to close overview window")
	}
	// Wait for the swipe-to-close animation to finish before dragging the divider. This is important
	// because the state at the beginning of the divider drag determines which performance metrics
	// are recorded for the entire drag. If the drag begins before the window has actually closed,
	// the resulting data will be for Ash.SplitViewResize.PresentationTime.TabletMode.WithOverview
	// and not Ash.SplitViewResize.PresentationTime.TabletMode.SingleWindow.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}

	// 5. Drag the divider.
	testing.ContextLog(ctx, "Dragging the divider between an empty overview grid and a snapped window")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, slow); err != nil {
		return errors.Wrap(err, "failed to drag divider between empty overview grid and snapped window")
	}

	return nil
}

// RunTablet runs window arrangement cuj for tablet. Since windows are always
// maximized in tablet mode, we only test performance for tab dragging and split
// view resizing.
func RunTablet(ctx, closeCtx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context, act *arc.Activity, withTestVideo arc.ActivityStartOption) error {
	const (
		timeout           = 10 * time.Second
		doubleTapInterval = 100 * time.Millisecond
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
		uiauto.Sleep(time.Second),
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

	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kw.Close()
	topRow, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the top-row layout")
	}
	enterOverview := kw.AccelAction(topRow.SelectTask)
	// Exercise split view resize functionality.
	if err := exerciseSplitViewResize(ctx, tconn, ui, pc, splitViewDragPoints, enterOverview); err != nil {
		return errors.Wrap(err, "failed to exercise split view resize functionality with two browser windows")
	}

	// For the part with an ARC window, adjust the drag points to help avoid https://crbug.com/1297297.
	// Specifically, avoid resizing the ARC window to its minimum width which is 342. The split view
	// divider width is 8, half of that is 4, and so there are 4 DIPs between the divider's centerpoint
	// and the ARC window. The divider's centerpoint is between pixels and cannot be the exact position
	// of a touch gesture, so there are only 3 DIPs between the drag point and the ARC window. So the
	// ARC window has width 342 at a drag point 345 away from the right end. To avoid reaching the
	// minimum size of the ARC window, we stay 346 away from the right end.
	// TODO(https://crbug.com/1297297): Remove this when the bug is fixed.
	splitViewDragPoints[2].X -= 346

	// Start the ARC app.
	if err := act.Start(ctx, tconn, withTestVideo); err != nil {
		return errors.Wrap(err, "failed to start ARC app")
	}
	// Close the ARC app at the end of the test. Otherwise it will cause
	// the test server's Close() function to block for a few minutes.
	defer act.Stop(closeCtx, tconn)
	// The ARC app will be automatically snapped because split view mode is active.
	if err := ash.WaitForARCAppWindowState(ctx, tconn, pkgName, ash.WindowStateLeftSnapped); err != nil {
		return errors.Wrap(err, "failed to wait for ARC app to be snapped on left")
	}

	// Swap the windows so that enterOverview will put the browser
	// window in overview and leave the ARC window snapped.
	tapDivider := pc.ClickAt(splitViewDragPoints[0])
	if err := action.Combine("double tap the divider", tapDivider, uiauto.Sleep(doubleTapInterval), tapDivider)(ctx); err != nil {
		return errors.Wrap(err, "failed to swap snapped windows")
	}

	// Wait for location-change events to be completed.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}

	// Exercise split view resize functionality.
	if err := exerciseSplitViewResize(ctx, tconn, ui, pc, splitViewDragPoints, enterOverview); err != nil {
		return errors.Wrap(err, "failed to exercise split view resize functionality with an ARC window and a browser window")
	}

	return nil
}
