// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mouse injects mouse events via Chrome autotest private API.
package mouse

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// Button specifies a button on mouse.
type Button string

// As defined in Button in
// https://cs.chromium.org/chromium/src/chrome/common/extensions/api/autotest_private.idl?l=90
const (
	LeftButton   Button = "Left"
	RightButton         = "Right"
	MiddleButton        = "Middle"
)

// Click returns an func which causes a mouse click event. The location is relative to the top-left of
// the display.
func Click(tconn *chrome.TestConn, location coords.Point, button Button) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := Move(tconn, location, 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the target location")
		}
		return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.mouseClick)", button)
	}
}

// DoubleClick returns an func which causes 2 mouse click events with an given interval. The location is relative to the top-left of
// the display.
func DoubleClick(tconn *chrome.TestConn, location coords.Point, doubleClickInterval time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := Move(tconn, location, 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the target location")
		}
		if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.mouseClick)", LeftButton); err != nil {
			return err
		}
		if err := testing.Sleep(ctx, doubleClickInterval); err != nil {
			return errors.Wrap(err, "failed to wait for the gap between the double click")
		}
		return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.mouseClick)", LeftButton)
	}
}

// Press returns an func which requests a mouse press event on the current location of the mouse cursor.
// Ash will consider the button stays pressed, until release is requested.
func Press(tconn *chrome.TestConn, button Button) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.mousePress)", button)
	}
}

// Release returns an func which requests a release event of a mouse button. It will do nothing
// when the button is not pressed.
func Release(tconn *chrome.TestConn, button Button) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.mouseRelease)", button)
	}
}

// Move returns an func which requests to move the mouse cursor to a certain location. The
// location is relative to the top-left of the display. It does not support to
// move across multiple displays. When duration is 0, it moves instantly to the
// specified location. Otherwise, the cursor should move linearly during the
// period. Returns after the move event is handled by Ash.
// Note: If you want to move to a node, please use MouseMoveTo function defined in uiauto/automation.go.
func Move(tconn *chrome.TestConn, location coords.Point, duration time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.mouseMove)", location, duration/time.Millisecond)
	}
}

// Drag returns an func which is a helper function to cause a drag of the left button from start
// to end. The duration is the time between the movements from the start to the
// end (i.e. the duration of the drag), and the movement to the start happens
// instantly.
func Drag(tconn *chrome.TestConn, start, end coords.Point, duration time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := Move(tconn, start, 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start location")
		}
		if err := Press(tconn, LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the button")
		}
		if err := Move(tconn, end, duration)(ctx); err != nil {
			return errors.Wrap(err, "failed to drag")
		}
		return Release(tconn, LeftButton)(ctx)
	}
}
