// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
)

// MouseButton specifies a button on mouse.
type MouseButton string

// As defined in MouseButton in
// https://cs.chromium.org/chromium/src/chrome/common/extensions/api/autotest_private.idl?l=90
const (
	LeftButton   MouseButton = "Left"
	RightButton              = "Right"
	MiddleButton             = "Middle"
)

// MouseClick causes a click event. The location is relative to the top-left of
// the display.
func MouseClick(ctx context.Context, tconn *chrome.TestConn, location coords.Point, button MouseButton) error {
	if err := MouseMove(ctx, tconn, location, 0); err != nil {
		return errors.Wrap(err, "failed to move to the target location")
	}
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mouseClick)(%q)`, button)
	return tconn.EvalPromise(ctx, expr, nil)
}

// MousePress requests a press event on the current location of the mouse cursor.
// Ash will consider the button stays pressed, until release is requested.
func MousePress(ctx context.Context, tconn *chrome.TestConn, button MouseButton) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mousePress)(%q)`, button), nil)
}

// MouseRelease requests a release event of a mouse button. It will do nothing
// when the button is not pressed.
func MouseRelease(ctx context.Context, tconn *chrome.TestConn, button MouseButton) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mouseRelease)(%q)`, button), nil)
}

// MouseMove requests to move the mouse cursor to a certain location. The
// location is relative to the top-left of the display. It does not support to
// move across multiple displays. When duration is 0, it moves instantly to the
// specified location. Otherwise, the cursor should move linearly during the
// period. Returns after the move event is handled by Ash.
func MouseMove(ctx context.Context, tconn *chrome.TestConn, location coords.Point, duration time.Duration) error {
	locationData, err := json.Marshal(location)
	if err != nil {
		return err
	}
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mouseMove)(%s, %d)`, string(locationData), duration/time.Millisecond)
	return tconn.EvalPromise(ctx, expr, nil)
}

// MouseDrag is a helper function to cause a drag of the left button from start
// to end. The duration is the time between the movements from the start to the
// end (i.e. the duration of the drag), and the movement to the start happens
// instantly.
func MouseDrag(ctx context.Context, tconn *chrome.TestConn, start, end coords.Point, duration time.Duration) error {
	if err := MouseMove(ctx, tconn, start, 0); err != nil {
		return errors.Wrap(err, "failed to move to the start location")
	}
	if err := MousePress(ctx, tconn, LeftButton); err != nil {
		return errors.Wrap(err, "failed to press the button")
	}
	if err := MouseMove(ctx, tconn, end, duration); err != nil {
		return errors.Wrap(err, "failed to drag")
	}
	return MouseRelease(ctx, tconn, LeftButton)
}
