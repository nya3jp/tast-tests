// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

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

// WindowBound use incremental integers to represent window boundaries.
type WindowBound int

const (
	// TopLeft represents app's TopLeft corner.
	TopLeft WindowBound = iota
	// TopRight represents app's TopRight corner.
	TopRight
	// BottomLeft represents app's BottomLeft corner.
	BottomLeft
	// BottomRight represents app's BottomRight corner.
	BottomRight
	// Left represents app's Left edge.
	Left
	// Right represents app's Right edge.
	Right
	// Top represents app's Top edge.
	Top
	// Bottom represents app's Bottom edge.
	Bottom
)

// AllBounds returns all the bounds of the window.
func AllBounds() []WindowBound {
	return []WindowBound{
		TopLeft, TopRight, BottomLeft, BottomRight, Left, Right, Top, Bottom,
	}
}

// defaultMargin indicates the distance in pixels outside the border of the "normal" window from which it should be grabbed.
const defaultMargin = 5

// ResizeWindowUtil represents resize window utility.
type ResizeWindowUtil struct {
	tconn      *chrome.TestConn
	ui         *uiauto.Context
	resizeArea coords.Rect
}

// NewResizeUtil returns a new resize window utility entity.
func NewResizeUtil(ctx context.Context, tconn *chrome.TestConn) (*ResizeWindowUtil, error) {
	ui := uiauto.New(tconn)

	rootWindowFinder := nodewith.HasClass("RootWindow-0").Role(role.Window)
	resizeArea, err := ui.Info(ctx, rootWindowFinder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root window info")
	}

	shelfInfo, err := ui.Info(ctx, nodewith.Role(role.Toolbar).ClassName("ShelfView"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shelf info")
	}
	resizeArea.Location.Height -= shelfInfo.Location.Height

	return &ResizeWindowUtil{
		tconn:      tconn,
		ui:         ui,
		resizeArea: resizeArea.Location,
	}, nil
}

// stableDrag drags the window and waits for location be stabled.
func (t *ResizeWindowUtil) stableDrag(node *nodewith.Finder, srcPt, endPt coords.Point) uiauto.Action {
	return uiauto.Combine("mouse drag and wait for location be stabled",
		t.ui.WaitForLocation(node),
		mouse.Drag(t.tconn, srcPt, endPt, time.Second),
		t.ui.WaitForLocation(node),
	)
}

// ResizeWindowToMin minimizes window by dragging form top-left to bottom-right of the window.
func (t *ResizeWindowUtil) ResizeWindowToMin(f *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		if err := t.ui.WaitForLocation(f)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		rect, err := t.ui.Info(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to get window info")
		}

		return t.stableDrag(f, rect.Location.TopLeft(), rect.Location.BottomRight())(ctx)
	}
}

// MoveWindowToCenter places the window on the center of screen.
func (t *ResizeWindowUtil) MoveWindowToCenter(app *ResizeApp) uiauto.Action {
	return func(ctx context.Context) error {
		f := app.WindowFinder

		if err := t.ui.WaitForLocation(f)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		windowInfo, err := t.ui.Info(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to get window info")
		}

		src := coords.NewPoint(windowInfo.Location.CenterX(), windowInfo.Location.Top+defaultMargin)
		if app.IsArcApp {
			centerBtnInfo, err := t.ui.Info(ctx, nodewith.Name("Resizable").HasClass("FrameCenterButton"))
			if err != nil {
				testing.ContextLog(ctx, "Failed to get center button of title bar info")
			} else {
				// Drag the left side of center button of the title bar to avoid moving failure.
				src = coords.Point{X: centerBtnInfo.Location.Left - defaultMargin, Y: windowInfo.Location.Top + defaultMargin}
			}
		}
		dest := coords.NewPoint(t.resizeArea.CenterX(), t.resizeArea.CenterY()-windowInfo.Location.Height/2)

		return t.stableDrag(f, src, dest)(ctx)
	}
}

// ResizeWindow resizes window by dragging corners/sides.
func (t *ResizeWindowUtil) ResizeWindow(f *nodewith.Finder, dragBound WindowBound) uiauto.Action {
	return func(ctx context.Context) error {
		if err := t.ui.WaitForLocation(f)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		windowInfoBefore, err := t.ui.Info(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to get window info before resize")
		}

		start, end, err := t.dragDetail(dragBound, windowInfoBefore)
		if err != nil {
			return errors.Wrap(err, "failed to get drag detail")
		}

		if err := t.stableDrag(f, start, end)(ctx); err != nil {
			return errors.Wrap(err, "failed to resize")
		}

		windowInfoAfter, err := t.ui.Info(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to get window info after resize")
		}

		if !t.verifyLocation(ctx, windowInfoAfter.Location, end, dragBound) {
			return errors.New("failed to verify that the window has been resized")
		}
		testing.ContextLog(ctx, "Window has resized as expected")

		if err := t.stableDrag(f, end, start)(ctx); err != nil {
			return errors.Wrap(err, "failed to restore resize")
		}

		return nil
	}
}

// verifyLocation verifies that the new position of window is correct.
func (t *ResizeWindowUtil) verifyLocation(ctx context.Context, windowLoc coords.Rect, expectedPos coords.Point, dragBound WindowBound) bool {
	// The difference between each bound of target window's location and expected position must be not greater than a threshold.
	const maxBias = defaultMargin

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
func (t *ResizeWindowUtil) dragDetail(dragBound WindowBound, nodeToResize *uiauto.NodeInfo) (sourcePt, endPt coords.Point, err error) {
	var shift coords.Point
	switch dragBound {
	case TopLeft:
		shift = coords.NewPoint(-defaultMargin, -defaultMargin)
		sourcePt = nodeToResize.Location.TopLeft()
		endPt = t.resizeArea.TopLeft()
	case TopRight:
		shift = coords.NewPoint(defaultMargin, -defaultMargin)
		sourcePt = nodeToResize.Location.TopRight()
		endPt = t.resizeArea.TopRight()
	case BottomLeft:
		shift = coords.NewPoint(-defaultMargin, defaultMargin)
		sourcePt = nodeToResize.Location.BottomLeft()
		endPt = t.resizeArea.BottomLeft()
	case BottomRight:
		shift = coords.NewPoint(defaultMargin, defaultMargin)
		sourcePt = nodeToResize.Location.BottomRight()
		endPt = t.resizeArea.BottomRight()
	case Left:
		shift = coords.NewPoint(-defaultMargin, 0)
		sourcePt = nodeToResize.Location.LeftCenter()
		endPt = t.resizeArea.LeftCenter()
		endPt.Y = sourcePt.Y
	case Right:
		shift = coords.NewPoint(defaultMargin, 0)
		sourcePt = nodeToResize.Location.RightCenter()
		endPt = t.resizeArea.RightCenter()
		endPt.Y = sourcePt.Y
	case Top:
		shift = coords.NewPoint(0, -defaultMargin)
		sourcePt = coords.NewPoint(nodeToResize.Location.CenterX(), nodeToResize.Location.Top)
		endPt = coords.NewPoint(t.resizeArea.CenterX(), t.resizeArea.Top)
		endPt.X = sourcePt.X
	case Bottom:
		shift = coords.NewPoint(0, defaultMargin)
		sourcePt = nodeToResize.Location.BottomCenter()
		endPt = t.resizeArea.BottomCenter()
		endPt.X = sourcePt.X
	default:
		return sourcePt, endPt, errors.Errorf("unexpected drag bound: %v", dragBound)
	}
	return sourcePt.Add(shift), endPt.Sub(shift), nil
}
