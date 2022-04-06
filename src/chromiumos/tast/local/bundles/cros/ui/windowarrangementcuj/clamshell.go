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
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// multiresize summons a multiresizer and drags it like Drag, but with all
// drag points adjusted for the location of the multiresizer.
func multiresize(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context, dragPoints DragPoints, duration time.Duration) error {
	if err := action.Combine(
		"hover mouse where windows meet",
		mouse.Move(tconn, dragPoints[0].Sub(coords.NewPoint(10, 10)), 0),
		mouse.Move(tconn, dragPoints[0], duration),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to summon multiresizer")
	}

	multiresizerBounds, err := ui.Location(ctx, nodewith.Role("window").ClassName("MultiWindowResizeController"))
	if err != nil {
		return errors.Wrap(err, "failed to get the multiresizer location")
	}

	offset := multiresizerBounds.CenterPoint().Sub(dragPoints[0])
	var multiresizeDragPoints DragPoints
	for i, p := range dragPoints {
		multiresizeDragPoints[i] = p.Add(offset)
	}

	if err := Drag(ctx, tconn, pc, multiresizeDragPoints, duration); err != nil {
		return errors.Wrap(err, "failed to drag multiresizer")
	}

	return nil
}

// RunClamShell runs window arrangement cuj for clamshell. We test performance
// for resizing window, dragging window, maximizing window, minimizing window
// and split view resizing.
func RunClamShell(ctx, closeCtx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, pc pointer.Context, act *arc.Activity, withTestVideo arc.ActivityStartOption) error {
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

	splitViewDragPoints := DragPoints{
		info.WorkArea.CenterPoint(),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width/4, info.WorkArea.CenterY()),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width-1, info.WorkArea.CenterY()),
	}
	snapLeftPoint := coords.NewPoint(info.WorkArea.Left+1, info.WorkArea.CenterY())
	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

	// Resize window.
	bounds := browserWin.BoundsInRoot
	upperLeftPt := coords.NewPoint(bounds.Left, bounds.Top)
	middlePt := coords.NewPoint(bounds.Left+bounds.Width/2, bounds.Top+bounds.Height/2)
	testing.ContextLog(ctx, "Resizing the browser window")
	if err := pc.Drag(upperLeftPt, pc.DragTo(middlePt, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to resize browser window from the upper left to the middle")
	}

	browserWin, err = ash.GetWindow(ctx, tconn, browserWinID)
	if err != nil {
		return errors.Wrap(err, "failed to get browser window info")
	}
	bounds = browserWin.BoundsInRoot
	newUpperLeftPt := coords.NewPoint(bounds.Left, bounds.Top)
	if err := pc.Drag(newUpperLeftPt, pc.DragTo(upperLeftPt, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to resize browser window back from the middle")
	}

	// Drag window.
	newTabButton := nodewith.Name("New Tab")
	newTabButtonRect, err := ui.Location(ctx, newTabButton)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of the new tab button")
	}
	tabStripGapPt := coords.NewPoint(newTabButtonRect.Right()+10, newTabButtonRect.Top)
	testing.ContextLog(ctx, "Dragging the browser window")
	if err := pc.Drag(tabStripGapPt, pc.DragTo(middlePt, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag browser window from the tab strip point to the middle")
	}
	if err := pc.Drag(middlePt, pc.DragTo(tabStripGapPt, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag browser window back from the middle")
	}

	// Maximize window.
	maximizeButton := nodewith.Name("Maximize").ClassName("FrameCaptionButton").Role(role.Button)
	if err := ui.LeftClick(maximizeButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to maximize the browser window")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browserWinID && w.State == ash.WindowStateMaximized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for browser window to become maximized")
	}

	// Minimize window.
	minimizeButton := nodewith.Name("Minimize").ClassName("FrameCaptionButton").Role(role.Button)
	if err := ui.LeftClick(minimizeButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to minimize the browser window")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browserWinID && w.State == ash.WindowStateMinimized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for browser window to become minimized")
	}

	// Restore window.
	if _, err := ash.SetWindowState(ctx, tconn, browserWinID, ash.WMEventNormal, true /* waitForStateChange */); err != nil {
		return errors.Wrap(err, "failed to set the browser window state to normal")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browserWinID && w.State == ash.WindowStateNormal && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for browser window to become normal")
	}

	// Lacros browser sometime restores to a different bounds so calculate
	// a new grab point.
	newBrowserWin, err := ash.GetWindow(ctx, tconn, browserWinID)
	if err != nil {
		return errors.Wrap(err, "failed to get browser window info")
	}
	newBounds := newBrowserWin.BoundsInRoot
	tabStripGapPt = coords.NewPoint(newBounds.Left+newBounds.Width*3/4, newBounds.Top+10)

	// Snap the window to the left and drag the second tab to snap to the right.
	testing.ContextLog(ctx, "Snapping the browser window to the left")
	if err := pc.Drag(tabStripGapPt, pc.DragTo(snapLeftPoint, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to snap the browser window to the left")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browserWinID && w.State == ash.WindowStateLeftSnapped && !w.IsAnimating
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for browser window to be left snapped")
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
		return errors.New("browser windows are not snapped yet")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for browser windows to be snapped correctly")
	}

	// Use multiresize on the two snapped windows.
	testing.ContextLog(ctx, "Multiresizing two snapped browser windows")
	const dividerDragError = "failed to drag divider slightly left, all the way right, and back to center"
	if err := multiresize(ctx, tconn, ui, pc, splitViewDragPoints, duration); err != nil {
		return errors.Wrap(err, dividerDragError)
	}

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
	enterOverview := kw.AccelAction(topRow.SelectTask)
	if err := enterOverview(ctx); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}
	// Create a second virtual desk.
	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to create a new desk")
	}
	defer ash.CleanUpDesks(closeCtx, tconn)
	// Wait for location-change events to be completed.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}
	// Drag the first window from overview grid to snap.
	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the browser window in the overview mode")
	}
	if err := pc.Drag(w.OverviewInfo.Bounds.CenterPoint(), pc.DragTo(snapLeftPoint, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag browser window from overview to snap")
	}
	// Wait for location-change events to be completed.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}
	// Drag divider.
	testing.ContextLog(ctx, "Dragging the divider between a snapped browser window and an overview window")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, duration); err != nil {
		return errors.Wrap(err, dividerDragError)
	}
	// Drag the second window to another desk to obtain an empty overview grid.
	w, err = ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the window in the overview mode to drag to another desk")
	}
	deskMiniViews, err := ui.NodesInfo(ctx, nodewith.ClassName("DeskMiniView"))
	if err != nil {
		return errors.Wrap(err, "failed to get desk mini-views")
	}
	if deskMiniViewCount := len(deskMiniViews); deskMiniViewCount < 2 {
		return errors.Wrapf(err, "expected more than 1 desk mini-views; found %v", deskMiniViewCount)
	}
	if err := pc.Drag(w.OverviewInfo.Bounds.CenterPoint(),
		pc.DragTo(deskMiniViews[1].Location.CenterPoint(), duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag browser window from overview grid to desk mini-view")
	}
	// Wait for location-change events to be completed.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}
	// Drag divider.
	testing.ContextLog(ctx, "Dragging the divider between a snapped browser window and an empty overview grid")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, duration); err != nil {
		return errors.Wrap(err, dividerDragError)
	}

	// For the part with an ARC window, adjust the drag points to help avoid https://crbug.com/1297297.
	// Specifically, avoid resizing either window to its minimum width. The minimum width of the
	// browser window is 500, so we stay 501 away from the left end. Likewise, the minimum width of the
	// ARC window is 342, so we stay 343 away from the right end.
	// TODO(https://crbug.com/1297297): Remove this when the bug is fixed.
	if splitViewDragPoints[1].X < 501 {
		splitViewDragPoints[1].X = 501
	}
	splitViewDragPoints[2].X -= 343

	// Start the ARC app.
	if err := act.Start(ctx, tconn, withTestVideo); err != nil {
		return errors.Wrap(err, "failed to start ARC app")
	}
	// Close the ARC app at the end of the test. Otherwise it will cause
	// the test server's Close() function to block for a few minutes.
	defer act.Stop(closeCtx, tconn)
	// Use Alt+] to snap the ARC app on the right.
	if err := kw.AccelAction("Alt+]")(ctx); err != nil {
		return errors.Wrap(err, "failed to press Alt+] to snap the ARC app on the right")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, pkgName, ash.WindowStateRightSnapped); err != nil {
		return errors.Wrap(err, "failed to wait for ARC app to be snapped on right")
	}
	// Use multiresize on the two snapped windows.
	testing.ContextLog(ctx, "Multiresizing a snapped browser window and a snapped ARC window")
	if err := multiresize(ctx, tconn, ui, pc, splitViewDragPoints, duration); err != nil {
		return errors.Wrap(err, dividerDragError)
	}
	// Enter the overview mode.
	if err := enterOverview(ctx); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}
	// Wait for location-change events to be completed.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}
	// Drag the ARC window from overview grid to snap.
	w, err = ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return errors.Wrap(err, "failed to find ARC window in the overview mode")
	}
	if err := pc.Drag(w.OverviewInfo.Bounds.CenterPoint(), pc.DragTo(snapLeftPoint, duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag ARC window from overview to snap")
	}
	// Wait for location-change events to be completed.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}
	// Drag divider.
	testing.ContextLog(ctx, "Dragging the divider between a snapped ARC window and an overview window")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, duration); err != nil {
		return errors.Wrap(err, dividerDragError)
	}
	// Drag the remaining browser window to another desk to obtain an empty overview grid.
	w, err = ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the browser window in the overview mode to drag to another desk")
	}
	if err := pc.Drag(w.OverviewInfo.Bounds.CenterPoint(),
		pc.DragTo(deskMiniViews[1].Location.CenterPoint(), duration))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag browser window from overview grid to desk mini-view")
	}
	// Wait for location-change events to be completed.
	if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be completed")
	}
	// Drag divider.
	testing.ContextLog(ctx, "Dragging the divider between a snapped ARC window and an empty overview grid")
	if err := Drag(ctx, tconn, pc, splitViewDragPoints, duration); err != nil {
		return errors.Wrap(err, dividerDragError)
	}

	return nil
}
