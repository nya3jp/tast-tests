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

func (tc *Context) tapAt(ctx context.Context, loc coords.Point) error {
	stw, err := tc.tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to get the single touch writer")
	}
	defer stw.Close()

	x, y := tc.tcc.ConvertLocation(loc)
	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move the single touch")
	}
	stw.Close()
	return nil
}

// Tap returns a function that causes a tap the node through the touchscreen.
func (tc *Context) Tap(finder *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		loc, err := tc.ac.Location(ctx, finder)
		if err != nil {
			return errors.Wrap(err, "failed to get the location of the node")
		}
		return tc.tapAt(ctx, loc.CenterPoint())
	}
}

// TapAt returns a function that causes a tap on the specified location.
func (tc *Context) TapAt(loc coords.Point) uiauto.Action {
	return func(ctx context.Context) error {
		return tc.tapAt(ctx, loc)
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

type swipeContextKey struct{}

type swipeContext struct {
	stw  *input.SingleTouchEventWriter
	prev coords.Point
}

// SwipeTo returns a function to swipe to the target location in the duration.
// This should be used within Swipe method.
func (tc *Context) SwipeTo(p coords.Point, duration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		swipe, ok := ctx.Value(swipeContextKey{}).(*swipeContext)
		if !ok || swipe == nil {
			return errors.New("not in swipe context")
		}
		px, py := tc.tcc.ConvertLocation(swipe.prev)
		x, y := tc.tcc.ConvertLocation(p)
		swipe.prev = p
		return swipe.stw.Swipe(ctx, px, py, x, y, duration)
	}
}

// SwipeToNode returns a function to swipe to the target node in the duration.
// This should be used within Swipe method.
func (tc *Context) SwipeToNode(f *nodewith.Finder, duration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		swipe, ok := ctx.Value(swipeContextKey{}).(*swipeContext)
		if !ok || swipe == nil {
			return errors.New("not in swipe context")
		}
		loc, err := tc.ac.Location(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to find the location")
		}
		p := loc.CenterPoint()
		px, py := tc.tcc.ConvertLocation(swipe.prev)
		x, y := tc.tcc.ConvertLocation(p)
		swipe.prev = p
		return swipe.stw.Swipe(ctx, px, py, x, y, duration)
	}
}

// Swipe returns a function to initiate the single-touch gesture, and conducts
// the specified gesture actions. The gesture starts from the specified location.
// Examples:
//   // swipe from a location to another in a second.
//   tc.Swipe(start, tc.SwipeTo(end, time.Second))
//
//   // Longpress and swipe.
//   tc.Swipe(start, func(ctx context.Context) error { return testing.Sleep(ctx, 2*time.Second) }, tc.SwipeTo(end, time.Second))
//
//   // Multiple points.
//   tc.Swipe(points[0], tc.SwipeTo(points[1], time.Second), tc.SwipeTo(points[2], time.Second))
// Swipe returns a function to cause a swipe from a point to the other in duration.
func (tc *Context) Swipe(loc coords.Point, gestures ...uiauto.Action) uiauto.Action {
	gestureAction := uiauto.Combine("swipe gesture", gestures...)
	return func(ctx context.Context) error {
		stw, err := tc.tsw.NewSingleTouchWriter()
		if err != nil {
			return errors.Wrap(err, "failed to get the single touch writer")
		}
		defer stw.Close()

		swipe := &swipeContext{stw: stw, prev: loc}
		ctx = context.WithValue(ctx, swipeContextKey{}, swipe)

		x, y := tc.tcc.ConvertLocation(loc)
		if err := stw.Move(x, y); err != nil {
			return errors.Wrap(err, "failed to move to the initial location")
		}

		return gestureAction(ctx)
	}
}
