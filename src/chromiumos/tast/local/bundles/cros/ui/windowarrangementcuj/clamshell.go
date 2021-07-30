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
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// RunClamShell runs window arrangement cuj for clamshell. We test performance
// for resizing window, dragging window, maximizing window, minimizing window
// and split view resizing.
func RunClamShell(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context) error {
	const (
		timeout  = 10 * time.Second
		duration = 2 * time.Second
	)

	// Gets info of the browser window, assuming it is the active window.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	browserWinID := ws[0].ID

	// Turn the window into normal state.
	if err := ash.SetWindowStateAndWait(ctx, tconn, browserWinID, ash.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to set the window state to normal")
	}

	browserWin, err := ash.GetWindow(ctx, tconn, browserWinID)
	if err != nil {
		return errors.Wrap(err, "failed to get window")
	}
	if browserWin.State != ash.WindowStateNormal {
		return errors.Errorf("Wrong window state: expected Normal, got %s", browserWin.State)
	}

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
	snapLeftPoint := coords.NewPoint(info.WorkArea.Left+1, info.WorkArea.CenterY())
	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

	// Resize window.
	bounds := browserWin.BoundsInRoot
	upperLeftPt := coords.NewPoint(bounds.Left, bounds.Top)
	middlePt := coords.NewPoint(bounds.Left+bounds.Width/2, bounds.Top+bounds.Height/2)
	testing.ContextLog(ctx, "Resizing the window")
	if err := pc.Drag(upperLeftPt, pc.DragTo(middlePt, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to resize window from the upper left to the middle")
	}

	browserWin, err = ash.GetWindow(ctx, tconn, browserWinID)
	if err != nil {
		return errors.Wrap(err, "failed to get window info")
	}
	bounds = browserWin.BoundsInRoot
	newUpperLeftPt := coords.NewPoint(bounds.Left, bounds.Top)
	if err := pc.Drag(newUpperLeftPt, pc.DragTo(upperLeftPt, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to resize window back from the middle")
	}

	// Drag window.
	newTabButton := nodewith.Name("New Tab")
	newTabButtonRect, err := ui.Location(ctx, newTabButton)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the new tab button")
	}
	tabStripGapPt := coords.NewPoint(newTabButtonRect.Right()+10, newTabButtonRect.Top)
	testing.ContextLog(ctx, "Dragging the window")
	if err := pc.Drag(tabStripGapPt, pc.DragTo(middlePt, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag window from the tab strip point to the middle")
	}
	if err := pc.Drag(middlePt, pc.DragTo(tabStripGapPt, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag window back from the middle")
	}

	// Maximize window.
	maximizeButton := nodewith.Name("Maximize").ClassName("FrameCaptionButton").Role(role.Button)
	if err := ui.LeftClick(maximizeButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to maximize the window")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browserWinID && w.State == ash.WindowStateMaximized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for window to become maximized")
	}

	// Minimize window.
	minimizeButton := nodewith.Name("Minimize").ClassName("FrameCaptionButton").Role(role.Button)
	if err := ui.LeftClick(minimizeButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to minimize the window")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browserWinID && w.State == ash.WindowStateMinimized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for window to become minimized")
	}

	// Restore window.
	if _, err := ash.SetWindowState(ctx, tconn, browserWinID, ash.WMEventNormal, true /* waitForStateChange */); err != nil {
		return errors.Wrap(err, "failed to set the window state to normal")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browserWinID && w.State == ash.WindowStateNormal && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for window to become normal")
	}

	// Lacros browser sometime restores to a different bounds so calculate
	// a new grab point.
	newBrowserWin, err := ash.GetWindow(ctx, tconn, browserWinID)
	if err != nil {
		return errors.Wrap(err, "failed to get window info")
	}
	newBounds := newBrowserWin.BoundsInRoot
	tabStripGapPt = coords.NewPoint(newBounds.Left+newBounds.Width*3/4, newBounds.Top+10)

	// Snap the window to the left and drag the second tab to snap to the right.
	testing.ContextLog(ctx, "Snapping the window to the left")
	if err := pc.Drag(tabStripGapPt, pc.DragTo(snapLeftPoint, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to snap the window to the left")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browserWinID && w.State == ash.WindowStateLeftSnapped && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for window to be left snapped")
	}
	testing.ContextLog(ctx, "Snapping the second tab to the right")
	firstTab := nodewith.Role(role.Tab).ClassName("Tab").First()
	firstTabRect, err := ui.Location(ctx, firstTab)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the first tab")
	}
	if err := pc.Drag(firstTabRect.CenterPoint(), pc.DragTo(snapRightPoint, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to snap the second tab to the right")
	}

	ws, err = ash.GetAllWindows(ctx, tconn)
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

	// Split view resizing. Some preparations need to be done before dragging the divider in
	// order to collect Ash.SplitViewResize.PresentationTime.SingleWindow. It must have a snapped
	// window and an overview grid to be able to collect the metrics for SplitViewController.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kw.Close()
	// Enter the overview mode.
	topRow, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the top-row layout")
	}
	if err = kw.Accel(ctx, topRow.SelectTask); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}
	// Snap one of the window to the left from the overview grid.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to create a new desk")
	}
	// Wait for location-change events to be completed.
	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}
	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the window in the overview mode")
	}
	// Drag the first window from overview grid to snap.
	if err := pc.Drag(w.OverviewInfo.Bounds.CenterPoint(), pc.DragTo(snapLeftPoint, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag window from overview to snap")
	}
	// Wait for location-change events to be completed.
	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}
	w, err = ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the window in the overview mode to drag to snap")
	}
	deskMiniViews, err := ui.NodesInfo(ctx, nodewith.ClassName("DeskMiniView"))
	if err != nil {
		return errors.Wrap(err, "failed to get desk mini-views")
	}
	if deskMiniViewCount := len(deskMiniViews); deskMiniViewCount < 2 {
		return errors.Wrapf(err, "expected more than 1 desk mini-views; found %v", deskMiniViewCount)
	}
	// Drag the second window to another desk to obtain an empty overview grid.
	if err := pc.Drag(w.OverviewInfo.Bounds.CenterPoint(),
		ui.Sleep(2*time.Second),
		pc.DragTo(deskMiniViews[1].Location.CenterPoint(), duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag window from overview grid to desk mini-view")
	}
	// Wait for location-change events to be completed.
	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}

	// Drag divider.
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
