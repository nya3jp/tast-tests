// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package touch provides the uiauto actions to control the touchscreen.
package touch

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Context provides the interface to the touchscreen.
type Context struct {
	ac  *uiauto.Context
	tsw *input.TouchscreenEventWriter
	tcc *input.TouchCoordConverter
}

// NewTouchscreenAndConverter is a utility to create a new touchscreen event
// writer and its touch coord converter with checking the display bounds.
func NewTouchscreenAndConverter(ctx context.Context, tconn *chrome.TestConn) (*input.TouchscreenEventWriter, *input.TouchCoordConverter, error) {
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to access to the touchscreen")
	}
	success := false
	defer func() {
		if !success {
			if err := tsw.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close the touchscreen: ", err)
			}
		}
	}()

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get the orientation information")
	}
	// Some devices (like kukui/krane) has rotated panel orientation. The negative
	// value of the panel orientation (rotation.Angle) should be set to cancel
	// this effect. See also: https://crbug.com/1022614.
	if err := tsw.SetRotation(-orientation.Angle); err != nil {
		return nil, nil, errors.Wrap(err, "failed to rotate the touchscreen event writer")
	}

	// Creates the TouchCoordConverter for the touch screen.  Here assumes that
	// the touch screen is the internal display.  This does not work if the
	// device does not have the internal touch display but has an external touch
	// display.
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get the internal display info")
	}
	success = true
	return tsw, tsw.NewTouchCoordConverter(info.Bounds.Size()), nil
}

// New creates a new instance of Context.
func New(ctx context.Context, tconn *chrome.TestConn) (*Context, error) {
	tsw, tcc, err := NewTouchscreenAndConverter(ctx, tconn)
	if err != nil {
		return nil, err
	}
	return &Context{tsw: tsw, tcc: tcc, ac: uiauto.New(tconn)}, nil
}

// Close closes the access to the touch screen.
func (tc *Context) Close() error {
	return tc.tsw.Close()
}

// WithTimeout returns a new Context with the specified timeout.
func (tc *Context) WithTimeout(timeout time.Duration) *Context {
	return &Context{
		tsw: tc.tsw,
		tcc: tc.tcc,
		ac:  tc.ac.WithTimeout(timeout),
	}
}

// WithInterval returns a new Context with the specified polling interval.
func (tc *Context) WithInterval(interval time.Duration) *Context {
	return &Context{
		tsw: tc.tsw,
		tcc: tc.tcc,
		ac:  tc.ac.WithInterval(interval),
	}
}

// WithPollOpts returns a new Context with the specified polling options.
func (tc *Context) WithPollOpts(pollOpts testing.PollOptions) *Context {
	return &Context{
		tsw: tc.tsw,
		tcc: tc.tcc,
		ac:  tc.ac.WithPollOpts(pollOpts),
	}
}

// Tap returns a function that causes a tap the node through the touchscreen.
func (tc *Context) Tap(finder *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		stw, err := tc.tsw.NewSingleTouchWriter()
		if err != nil {
			return errors.Wrap(err, "failed to get the single touch writer")
		}
		defer stw.Close()

		loc, err := tc.ac.Location(ctx, finder)
		if err != nil {
			return errors.Wrap(err, "failed to get the location of the node")
		}
		x, y := tc.tcc.ConvertLocation(loc.CenterPoint())
		if err := stw.Move(x, y); err != nil {
			return errors.Wrap(err, "failed to move the single touch")
		}
		stw.Close()
		return nil
	}
}

// LongPress returns a function that causes a long press at the node through the
// touchscreen.
func (tc *Context) LongPress(finder *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		stw, err := tc.tsw.NewSingleTouchWriter()
		if err != nil {
			return errors.Wrap(err, "failed to get the single touch writer")
		}
		defer stw.Close()

		loc, err := tc.ac.Location(ctx, finder)
		if err != nil {
			return errors.Wrap(err, "failed to get the location of the node")
		}
		x, y := tc.tcc.ConvertLocation(loc.CenterPoint())
		if err := stw.LongPressAt(ctx, x, y); err != nil {
			return errors.Wrap(err, "failed to move the single touch")
		}
		stw.Close()
		return nil
	}
}

func (tc *Context) swipeAction(ctx context.Context, from, to coords.Point, duration time.Duration) error {
	stw, err := tc.tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to get the single touch writer")
	}
	defer stw.Close()

	xf, yf := tc.tcc.ConvertLocation(from)
	xt, yt := tc.tcc.ConvertLocation(to)
	if err := stw.Swipe(ctx, xf, yf, xt, yt, duration); err != nil {
		return errors.Wrap(err, "failed to move the single touch")
	}
	stw.Close()
	return nil
}

// Swipe returns a function to cause a swipe from a point to the other in duration.
func (tc *Context) Swipe(from, to coords.Point, duration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		return tc.swipeAction(ctx, from, to, duration)
	}
}

// SwipeFrom returns a function to cause a swipe from the node to the specified
// point in duration.
func (tc *Context) SwipeFrom(finder *nodewith.Finder, toPoint coords.Point, duration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		loc, err := tc.ac.Location(ctx, finder)
		if err != nil {
			return errors.Wrap(err, "failed to find the location")
		}
		return tc.swipeAction(ctx, loc.CenterPoint(), toPoint, duration)
	}
}

// SwipeTo returns a function to cause a swipe from the specified location to
// the node in duration.
func (tc *Context) SwipeTo(finder *nodewith.Finder, fromPoint coords.Point, duration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		loc, err := tc.ac.Location(ctx, finder)
		if err != nil {
			return errors.Wrap(err, "failed to find the location")
		}
		return tc.swipeAction(ctx, fromPoint, loc.CenterPoint(), duration)
	}
}

// SwipeFromTo returns a function to cause a swipe from one node to another in
// duration.
func (tc *Context) SwipeFromTo(from, to *nodewith.Finder, duration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		fromLoc, err := tc.ac.Location(ctx, from)
		if err != nil {
			return errors.Wrap(err, "failed to find the from location")
		}
		toLoc, err := tc.ac.Location(ctx, to)
		if err != nil {
			return errors.Wrap(err, "failed to find the to location")
		}
		return tc.swipeAction(ctx, fromLoc.CenterPoint(), toLoc.CenterPoint(), duration)
	}
}
