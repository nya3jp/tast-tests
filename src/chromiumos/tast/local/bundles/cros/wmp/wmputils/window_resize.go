// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"
	"math"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/wmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// defaultMargin indicates the distance in pixels outside the border of the "normal" window from which it should be grabbed.
const defaultMargin = 5

// WindowBound use incremental integers to represent window boundaries.
type WindowBound string

const (
	// TopLeft represents app's TopLeft corner.
	TopLeft WindowBound = "TopLeft"
	// TopRight represents app's TopRight corner.
	TopRight WindowBound = "TopRight"
	// BottomLeft represents app's BottomLeft corner.
	BottomLeft WindowBound = "BottomLeft"
	// BottomRight represents app's BottomRight corner.
	BottomRight WindowBound = "BottomRight"
	// Left represents app's Left edge.
	Left WindowBound = "Left"
	// Right represents app's Right edge.
	Right WindowBound = "Right"
	// Top represents app's Top edge.
	Top WindowBound = "Top"
	// Bottom represents app's Bottom edge.
	Bottom WindowBound = "Bottom"
)

// AllBounds returns all the bounds of the window.
func AllBounds() []WindowBound {
	return []WindowBound{
		TopLeft, TopRight, BottomLeft, BottomRight, Left, Right, Top, Bottom,
	}
}

// ResizeApp represents an app that will be resized.
type ResizeApp struct {
	Name         string
	ID           string
	IsArcApp     bool
	WindowFinder *nodewith.Finder
}

// TurnOffWindowPreset sets the mode of ARC app from `Tablet` to `Resizable`.
// Non-ARC app will be ignored.
func (ra *ResizeApp) TurnOffWindowPreset(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	if !ra.IsArcApp {
		return nil
	}

	testing.ContextLog(ctx, "Setting ARC app window mode to `Resizable`")
	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/app-management/detail?id="+ra.ID)
	if err != nil {
		return errors.Wrapf(err, "failed to open settings of ARC app %q", ra.Name)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	ui := uiauto.New(tconn)

	btnResizeLock := nodewith.Name("Preset window sizes")
	if err := ui.WaitUntilExists(btnResizeLock)(ctx); err != nil {
		testing.ContextLogf(ctx, "ARC app %q should be resizable already", ra.Name)
		return nil
	}
	return ui.LeftClick(btnResizeLock)(ctx)
}

// ResizeWindowToMin minimizes window by dragging form top-left to bottom-right of the window.
func (ra *ResizeApp) ResizeWindowToMin(tconn *chrome.TestConn, window *nodewith.Finder) uiauto.Action {
	ui := uiauto.New(tconn)

	return func(ctx context.Context) error {
		if err := ui.WaitForLocation(window)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		rect, err := ui.Info(ctx, window)
		if err != nil {
			return errors.Wrap(err, "failed to get window info")
		}

		return wmp.StableDrag(tconn, window, rect.Location.TopLeft(), rect.Location.BottomRight())(ctx)
	}
}

// MoveWindowToCenter places the window on the center of screen.
// Window should NOT be maximized before this function.
func (ra *ResizeApp) MoveWindowToCenter(tconn *chrome.TestConn, window *nodewith.Finder, isArcApp bool) uiauto.Action {
	ui := uiauto.New(tconn)

	return func(ctx context.Context) error {
		if err := ui.WaitForLocation(window)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		windowInfo, err := ui.Info(ctx, window)
		if err != nil {
			return errors.Wrap(err, "failed to get window info")
		}

		src := coords.NewPoint(windowInfo.Location.CenterX(), windowInfo.Location.Top+defaultMargin)
		if isArcApp {
			centerBtnInfo, err := ui.Info(ctx, nodewith.Name("Resizable").HasClass("FrameCenterButton"))
			if err != nil {
				testing.ContextLog(ctx, "Failed to get center button of title bar info")
			} else {
				// Drag the left side of center button of the title bar to avoid moving failure.
				src = coords.Point{X: centerBtnInfo.Location.Left - defaultMargin, Y: windowInfo.Location.Top + defaultMargin}
			}
		}

		resizeArea, err := wmp.ResizableArea(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get resizable area")
		}

		dest := coords.NewPoint(resizeArea.CenterX(), resizeArea.CenterY()-windowInfo.Location.Height/2)

		return wmp.StableDrag(tconn, window, src, dest)(ctx)
	}
}

// ResizeWindow resizes window by dragging corners/sides.
// SetupResizableArea should be called prior to calling this function.
func (ra *ResizeApp) ResizeWindow(tconn *chrome.TestConn, window *nodewith.Finder, dragBound WindowBound) uiauto.Action {
	ui := uiauto.New(tconn)

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resizing window by dragging %q", dragBound)
		if err := ui.WaitForLocation(window)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		windowInfoBefore, err := ui.Info(ctx, window)
		if err != nil {
			return errors.Wrap(err, "failed to get window info before resize")
		}

		start, end, err := ra.dragDetail(ctx, tconn, dragBound, windowInfoBefore)
		if err != nil {
			return errors.Wrap(err, "failed to get drag detail")
		}

		testing.ContextLogf(ctx, "Dragging from %v to %v", start, end)

		if err := wmp.StableDrag(tconn, window, start, end)(ctx); err != nil {
			return errors.Wrap(err, "failed to resize")
		}

		windowInfoAfter, err := ui.Info(ctx, window)
		if err != nil {
			return errors.Wrap(err, "failed to get window info after resize")
		}

		if !ra.verifyLocation(ctx, windowInfoAfter.Location, end, dragBound) {
			return errors.Errorf("failed to verify that the window has been resized for bound %q: Before %q, after %q", dragBound, windowInfoBefore.Location, windowInfoAfter.Location)
		}

		testing.ContextLog(ctx, "Window has resized as expected")

		if err := wmp.StableDrag(tconn, window, end, start)(ctx); err != nil {
			return errors.Wrap(err, "failed to restore resize")
		}

		return nil
	}
}

// verifyLocation verifies that the new position of window is correct.
func (ra *ResizeApp) verifyLocation(ctx context.Context, windowLoc coords.Rect, expectedPos coords.Point, dragBound WindowBound) bool {
	// The difference between each bound of target window's location and expected position must be not greater than a threshold.
	// As StableDrag() preserves one defaultMargin for both sourcePt and endPt, the actual result might be different than what
	// we expected. The acceptable bias value would be less than (or equal to) 2 defaultMargin.
	const maxBias = 2 * defaultMargin

	testing.ContextLog(ctx, "Window location after resizing: ", windowLoc)

	switch dragBound {
	case TopLeft:
		return (math.Abs(float64(windowLoc.Left)-float64(expectedPos.X)) <= maxBias) && (math.Abs(float64(windowLoc.Top)-float64(expectedPos.Y)) <= maxBias)
	case TopRight:
		return (math.Abs(float64(windowLoc.Right())-float64(expectedPos.X)) <= maxBias) && (math.Abs(float64(windowLoc.Top)-float64(expectedPos.Y)) <= maxBias)
	case BottomLeft:
		return (math.Abs(float64(windowLoc.Left)-float64(expectedPos.X)) <= maxBias) && (math.Abs(float64(windowLoc.Bottom())-float64(expectedPos.Y)) <= maxBias)
	case BottomRight:
		return (math.Abs(float64(windowLoc.Right())-float64(expectedPos.X)) <= maxBias) && (math.Abs(float64(windowLoc.Bottom())-float64(expectedPos.Y)) <= maxBias)
	case Left:
		return math.Abs(float64(windowLoc.Left)-float64(expectedPos.X)) <= maxBias
	case Right:
		return math.Abs(float64(windowLoc.Right())-float64(expectedPos.X)) <= maxBias
	case Top:
		return math.Abs(float64(windowLoc.Top)-float64(expectedPos.Y)) <= maxBias
	case Bottom:
		return math.Abs(float64(windowLoc.Bottom())-float64(expectedPos.Y)) <= maxBias
	default:
		testing.ContextLog(ctx, "Unexpected drag bound: ", dragBound)
		return false
	}
}

// dragDetail adjusts the position that should be dragged according to defaultMargin.
func (ra *ResizeApp) dragDetail(ctx context.Context, tconn *chrome.TestConn, dragBound WindowBound, nodeToResize *uiauto.NodeInfo) (sourcePt, endPt coords.Point, err error) {
	resizeArea, err := wmp.ResizableArea(ctx, tconn)
	if err != nil {
		return sourcePt, endPt, errors.Wrap(err, "failed to get resizable area")
	}
	testing.ContextLog(ctx, "Resizable area is ", resizeArea.BottomRight())

	var shift coords.Point
	switch dragBound {
	case TopLeft:
		shift = coords.NewPoint(-defaultMargin, -defaultMargin)
		sourcePt = nodeToResize.Location.TopLeft()
		endPt = resizeArea.TopLeft()
	case TopRight:
		shift = coords.NewPoint(defaultMargin, -defaultMargin)
		sourcePt = nodeToResize.Location.TopRight()
		endPt = resizeArea.TopRight()
	case BottomLeft:
		shift = coords.NewPoint(-defaultMargin, defaultMargin)
		sourcePt = nodeToResize.Location.BottomLeft()
		endPt = resizeArea.BottomLeft()
	case BottomRight:
		shift = coords.NewPoint(defaultMargin, defaultMargin)
		sourcePt = nodeToResize.Location.BottomRight()
		endPt = resizeArea.BottomRight()
	case Left:
		shift = coords.NewPoint(-defaultMargin, 0)
		sourcePt = nodeToResize.Location.LeftCenter()
		endPt = resizeArea.LeftCenter()
		endPt.Y = sourcePt.Y
	case Right:
		shift = coords.NewPoint(defaultMargin, 0)
		sourcePt = nodeToResize.Location.RightCenter()
		endPt = resizeArea.RightCenter()
		endPt.Y = sourcePt.Y
	case Top:
		shift = coords.NewPoint(0, -defaultMargin)
		sourcePt = coords.NewPoint(nodeToResize.Location.CenterX(), nodeToResize.Location.Top)
		endPt = coords.NewPoint(resizeArea.CenterX(), resizeArea.Top)
		endPt.X = sourcePt.X
	case Bottom:
		shift = coords.NewPoint(0, defaultMargin)
		sourcePt = nodeToResize.Location.BottomCenter()
		endPt = resizeArea.BottomCenter()
		endPt.X = sourcePt.X
	default:
		return sourcePt, endPt, errors.Errorf("unexpected drag bound: %v", dragBound)
	}
	return sourcePt.Add(shift), endPt.Sub(shift), nil
}
