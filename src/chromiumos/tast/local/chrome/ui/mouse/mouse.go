// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mouse injects mouse events via Chrome autotest private API.
package mouse

import (
	"context"
	"encoding/json"
	"fmt"
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

// Click causes a mouse click event. The location is relative to the top-left of
// the display.
func Click(ctx context.Context, tconn *chrome.TestConn, location coords.Point, button Button) error {
	if err := Move(ctx, tconn, location, 0); err != nil {
		return errors.Wrap(err, "failed to move to the target location")
	}
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mouseClick)(%q)`, button)
	return tconn.EvalPromise(ctx, expr, nil)
}

// DoubleClick causes 2 mouse click events with an given interval. The location is relative to the top-left of
// the display.
func DoubleClick(ctx context.Context, tconn *chrome.TestConn, location coords.Point, doubleClickInterval time.Duration) error {
	if err := Move(ctx, tconn, location, 0); err != nil {
		return errors.Wrap(err, "failed to move to the target location")
	}
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mouseClick)(%q)`, LeftButton)
	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, doubleClickInterval); err != nil {
		return errors.Wrap(err, "failed to wait for the gap between the double click")
	}
	return tconn.EvalPromise(ctx, expr, nil)
}

// Press requests a mouse press event on the current location of the mouse cursor.
// Ash will consider the button stays pressed, until release is requested.
func Press(ctx context.Context, tconn *chrome.TestConn, button Button) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mousePress)(%q)`, button), nil)
}

// Release requests a release event of a mouse button. It will do nothing
// when the button is not pressed.
func Release(ctx context.Context, tconn *chrome.TestConn, button Button) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mouseRelease)(%q)`, button), nil)
}

// Move requests to move the mouse cursor to a certain location. The
// location is relative to the top-left of the display. It does not support to
// move across multiple displays. When duration is 0, it moves instantly to the
// specified location. Otherwise, the cursor should move linearly during the
// period. Returns after the move event is handled by Ash.
func Move(ctx context.Context, tconn *chrome.TestConn, location coords.Point, duration time.Duration) error {
	locationData, err := json.Marshal(location)
	if err != nil {
		return err
	}
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mouseMove)(%s, %d)`, string(locationData), duration/time.Millisecond)
	return tconn.EvalPromise(ctx, expr, nil)
}

// Drag is a helper function to cause a drag of the left button from start
// to end. The duration is the time between the movements from the start to the
// end (i.e. the duration of the drag), and the movement to the start happens
// instantly.
func Drag(ctx context.Context, tconn *chrome.TestConn, start, end coords.Point, duration time.Duration) error {
	if err := Move(ctx, tconn, start, 0); err != nil {
		return errors.Wrap(err, "failed to move to the start location")
	}
	if err := Press(ctx, tconn, LeftButton); err != nil {
		return errors.Wrap(err, "failed to press the button")
	}
	if err := Move(ctx, tconn, end, duration); err != nil {
		return errors.Wrap(err, "failed to drag")
	}
	return Release(ctx, tconn, LeftButton)
}
