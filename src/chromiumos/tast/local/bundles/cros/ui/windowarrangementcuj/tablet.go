// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package windowarrangementcuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
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

// exerciseSplitViewResize assumes two snapped windows and a second desk (but the
// first desk is the active one). Then exerciseSplitViewResize does the following:
// 1. Drag the divider.
// 2. Enter overview.
// 3. Drag the divider.
// 4. Drag the overview window to the second desk.
// 5. Drag the divider.
func exerciseSplitViewResize(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context, enterOverview action.Action, splitViewDragPoints ...coords.Point) error {
	const (
		slow              = 2 * time.Second
		moderatePace      = time.Second
		longPressDuration = time.Second
	)

	// 1. Drag the divider.
	testing.ContextLog(ctx, "Dragging the divider between two snapped windows")
	if err := dragAndRestore(ctx, tconn, pc, slow, splitViewDragPoints...); err != nil {
		return errors.Wrap(err, "failed to drag divider between two snapped windows")
	}

	// 2. Enter overview.
	if err := enterOverview(ctx); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}

	// 3. Drag the divider.
	testing.ContextLog(ctx, "Dragging the divider between an overview window and a snapped window")
	if err := dragAndRestore(ctx, tconn, pc, slow, splitViewDragPoints...); err != nil {
		return errors.Wrap(err, "failed to drag divider between overview window and snapped window")
	}

	// 4. Drag the overview window to the second desk.
	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the window in the overview mode to drag to the second desk")
	}
	deskMiniViews, err := ui.NodesInfo(ctx, nodewith.ClassName("DeskMiniView"))
	if err != nil {
		return errors.Wrap(err, "failed to get desk mini-views")
	}
	if deskMiniViewCount := len(deskMiniViews); deskMiniViewCount < 2 {
		return errors.Errorf("expected more than 1 desk mini-views; found %v", deskMiniViewCount)
	}
	if err := pc.Drag(
		// Initiate the drag with a long press at the centerpoint of the overview window.
		w.OverviewInfo.Bounds.CenterPoint(),
		uiauto.Sleep(longPressDuration),
		// Then drag the overview window to the centerpoint of the second desk mini-view.
		pc.DragTo(deskMiniViews[1].Location.CenterPoint(), moderatePace),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag overview window to second desk")
	}
	// Wait for the overview window to reach the second desk before dragging the divider. This is
	// important because the state at the beginning of the divider drag determines which performance
	// metrics are recorded for the entire drag. If the drag begins with a window still in overview,
	// the resulting data will be for Ash.SplitViewResize.PresentationTime.TabletMode.WithOverview
	// and not Ash.SplitViewResize.PresentationTime.TabletMode.SingleWindow.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}

	// 5. Drag the divider.
	testing.ContextLog(ctx, "Dragging the divider between an empty overview grid and a snapped window")
	if err := dragAndRestore(ctx, tconn, pc, slow, splitViewDragPoints...); err != nil {
		return errors.Wrap(err, "failed to drag divider between empty overview grid and snapped window")
	}

	return nil
}

// RunTablet runs window arrangement cuj for tablet. Since windows are always
// maximized in tablet mode, we only test performance for tab dragging and split
// view resizing.
func RunTablet(ctx, closeCtx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context) (retErr error) {
	const (
		timeout           = 10 * time.Second
		duration          = 2 * time.Second
		doubleTapInterval = 100 * time.Millisecond
	)

	// Gets primary display info and interesting drag points.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	splitViewDragPoints := []coords.Point{
		info.WorkArea.CenterPoint(),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width/4, info.WorkArea.CenterY()),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width-1, info.WorkArea.CenterY()),
	}
	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

	tabStripButton := nodewith.Role(role.Button).ClassName("WebUITabCounterButton").First()
	if err := pc.Click(tabStripButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click the tab strip button")
	}

	// Get the first tab location with a polling interval of 2 seconds (meaning
	// wait until the location is stable for 2 seconds) to work around a
	// glitchy animation that sometimes happens when bringing up the tab strip.
	firstTab := nodewith.Role(role.Tab).First()
	firstTabRect, err := ui.WithInterval(2*time.Second).Location(ctx, firstTab)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the first tab")
	}

	// Drag the first tab in the tab strip and snap it to the right.
	defer cleanUp(ctx, action.Named(
		"maximize the window",
		func(ctx context.Context) error {
			ws, err := getAllNonPipWindows(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get windows")
			}
			if len(ws) != 1 {
				return errors.Errorf("unexpected number of windows: got %d; want 1", len(ws))
			}
			if err := ash.SetWindowStateAndWait(ctx, tconn, ws[0].ID, ash.WindowStateMaximized); err != nil {
				return errors.Wrap(err, "failed to set browser window state to \"Maximized\"")
			}
			return nil
		},
	), &retErr)
	if err := pc.Drag(firstTabRect.CenterPoint(),
		uiauto.Sleep(time.Second),
		pc.DragTo(snapRightPoint, 3*time.Second),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag a tab to snap to the right")
	}
	defer cleanUp(ctx, action.Named(
		"recombine the browser tabs",
		func(ctx context.Context) error {
			return combineTabs(ctx, tconn, ui, pc, duration)
		},
	), &retErr)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := getAllNonPipWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain the window list")
		}
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

	// Create a second virtual desk.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to create a new desk")
	}
	defer cleanUp(closeCtx, action.Named(
		"remove extra desk",
		func(ctx context.Context) error {
			return removeExtraDesk(ctx, tconn)
		},
	), &retErr)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer cleanUp(closeCtx, action.Named(
		"close the keyboard",
		func(ctx context.Context) error {
			return kw.Close()
		},
	), &retErr)
	topRow, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the top-row layout")
	}
	enterOverview := kw.AccelAction(topRow.SelectTask)
	// Exercise split view resize functionality.
	if err := exerciseSplitViewResize(ctx, tconn, ui, pc, enterOverview, splitViewDragPoints...); err != nil {
		return errors.Wrap(err, "failed to exercise split view resize functionality with two browser windows")
	}

	return nil
}
