// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pointer provides utility interfaces to handle pointing devices (i.e.
// mouse and touch screen).
package pointer

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Controller provides the common interface to operate locations on
// screen, either by a mouse or the touch screen.
type Controller interface {
	// Press conducts an action to press at a location.
	Press(ctx context.Context, location coords.Point) error

	// Release cancels the press operation.
	Release(ctx context.Context) error

	// Move provides the movement event from start to end.
	Move(ctx context.Context, start, end coords.Point, duration time.Duration) error

	// Close closes the access to the underlying system and releases resources.
	Close()
}

// Click provides an action of press and release at the location, i.e. a mouse
// click or a tap of touch screen.
func Click(ctx context.Context, c Controller, location coords.Point) error {
	if err := c.Press(ctx, location); err != nil {
		return errors.Wrap(err, "failed to press")
	}
	return c.Release(ctx)
}

// Drag provides a drag action from start to the end.
func Drag(ctx context.Context, c Controller, start, end coords.Point, duration time.Duration) (err error) {
	if err = c.Press(ctx, start); err != nil {
		return errors.Wrap(err, "failed to press")
	}
	defer func() {
		releaseErr := c.Release(ctx)
		if releaseErr == nil {
			return
		}
		if err == nil {
			err = releaseErr
		} else {
			testing.ContextLog(ctx, "Failed to release: ", releaseErr)
		}
	}()
	err = c.Move(ctx, start, end, duration)
	return err
}

// MouseController implements Controller, conducted by a mouse.
type MouseController struct {
	tconn *chrome.TestConn
}

// NewMouseController creates a new MouseOperator.
func NewMouseController(tconn *chrome.TestConn) *MouseController {
	return &MouseController{tconn: tconn}
}

// Press implements Controller.Press.
func (mc *MouseController) Press(ctx context.Context, location coords.Point) error {
	if err := mouse.Move(mc.tconn, location, 0)(ctx); err != nil {
		return errors.Wrapf(err, "failed to move to the location: %v", location)
	}
	return mouse.Press(mc.tconn, mouse.LeftButton)(ctx)
}

// Release implements Controller.Release.
func (mc *MouseController) Release(ctx context.Context) error {
	return mouse.Release(mc.tconn, mouse.LeftButton)(ctx)
}

// Move implements Controller.Move.
func (mc *MouseController) Move(ctx context.Context, start, end coords.Point, duration time.Duration) error {
	if err := mouse.Move(mc.tconn, start, 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to the start location")
	}
	return mouse.Move(mc.tconn, end, duration)(ctx)
}

// Close implements Controller.Close.
func (mc *MouseController) Close() {
}

// TouchController implements Controller, conducted by a touch screen.
type TouchController struct {
	tsew *input.TouchscreenEventWriter
	stw  *input.SingleTouchEventWriter
	tcc  *input.TouchCoordConverter
}

// NewTouchController creates a TouchController on a new TouchscreenEventWriter.
func NewTouchController(ctx context.Context, tconn *chrome.TestConn) (*TouchController, error) {
	tsew, tcc, err := touch.NewTouchscreenAndConverter(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to access to the touch screen")
	}
	stw, err := tsew.NewSingleTouchWriter()
	if err != nil {
		tsew.Close()
		return nil, errors.Wrap(err, "failed to create the single touch writer")
	}
	return &TouchController{tsew: tsew, stw: stw, tcc: tcc}, nil
}

// Touchscreen returns the touchscreen for this controller.
func (tc *TouchController) Touchscreen() *input.TouchscreenEventWriter {
	return tc.tsew
}

// EventWriter returns the current single touch event writer for this controller.
func (tc *TouchController) EventWriter() *input.SingleTouchEventWriter {
	return tc.stw
}

// TouchCoordConverter returns the current TouchCoordConverter for this controller.
func (tc *TouchController) TouchCoordConverter() *input.TouchCoordConverter {
	return tc.tcc
}

// Press implements Controller.Press.
func (tc *TouchController) Press(ctx context.Context, location coords.Point) error {
	x, y := tc.tcc.ConvertLocation(location)
	return tc.stw.Move(x, y)
}

// Release implements Controller.Release.
func (tc *TouchController) Release(ctx context.Context) error {
	return tc.stw.End()
}

// Move implements Controller.Move.
func (tc *TouchController) Move(ctx context.Context, start, end coords.Point, duration time.Duration) error {
	startX, startY := tc.tcc.ConvertLocation(start)
	endX, endY := tc.tcc.ConvertLocation(end)
	return tc.stw.Swipe(ctx, startX, startY, endX, endY, duration)
}

// Close implements Controller.Close.
func (tc *TouchController) Close() {
	tc.stw.Close()
	tc.tsew.Close()
}
