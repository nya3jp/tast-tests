// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputsimulations

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// MoveMouseFor moves the mouse in a spiral according to r = theta, starting at the
// center of the screen. The mouse position is reset to the center of the screen once it
// reaches the border of the screen. Using this math function allows for both specifying
// a path for the mouse without defining coordinates in advance, as well as progressively
// increasing the speed that the mouse moves.
func MoveMouseFor(ctx context.Context, tconn *chrome.TestConn, duration time.Duration) error {
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	// Time to move the mouse between points.
	const deltaTime = 500 * time.Millisecond

	// How long it takes to spiral to the edge of the screen.
	const singleSpiralDuration = 2 * time.Minute

	// Maximum radius of the spiral before it is reset.
	maxRadius := math.Min(float64(info.Bounds.Width), float64(info.Bounds.Height)) / 2.0

	center := info.Bounds.CenterPoint()
	now := time.Now()
	endTime := now.Add(duration)
	for spiralStartTime := now; ; spiralStartTime = spiralStartTime.Add(singleSpiralDuration) {
		// Move mouse to the center of the screen.
		if err := mouse.Move(tconn, center, 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move mouse")
		}

		// Move the mouse in a spiral.
		spiralEndTime := spiralStartTime.Add(singleSpiralDuration)
		for {
			targetTime := time.Now().Add(deltaTime)
			if !targetTime.Before(endTime) {
				return nil
			} else if !targetTime.Before(spiralEndTime) {
				break
			}

			timeSinceSpiralStart := targetTime.Sub(spiralStartTime)
			a := float64(timeSinceSpiralStart) / float64(singleSpiralDuration)
			theta := a * maxRadius
			point := coords.NewPoint(
				center.X+int(theta*math.Cos(theta)),
				center.Y+int(theta*math.Sin(theta)))
			if err := mouse.Move(tconn, point, deltaTime)(ctx); err != nil {
				return errors.Wrap(err, "failed to move mouse")
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
		}
	}
}

// ScrollMouseDownFor rolls the scroll wheel for |duration|, with a |delay|
// between ticks.
func ScrollMouseDownFor(ctx context.Context, mw *input.MouseEventWriter, delay, duration time.Duration) error {
	if err := runActionFor(ctx, duration, action.Combine(
		"scroll down and sleep",
		func(ctx context.Context) error { return mw.ScrollDown() },
		action.Sleep(delay),
	)); err != nil {
		return errors.Wrap(err, "failed to scroll down repeatedly")
	}
	return nil
}

// RepeatMousePressFor presses the left mouse button at the current
// location for |pressDuration| and then releases it. This action
// is repeated until |totalDuration| has passed.
func RepeatMousePressFor(ctx context.Context, mw *input.MouseEventWriter, delay, pressDuration, totalDuration time.Duration) error {
	return runActionFor(ctx, totalDuration, action.Combine(
		"mouse press, sleep, and mouse release",
		func(ctx context.Context) error { return mw.Press() },
		action.Sleep(pressDuration),
		func(ctx context.Context) error { return mw.Release() },
		action.Sleep(delay),
	))
}

// RunDragMouseCycle presses the left mouse button at the center of the
// screen, and moves the mouse to the leftmost side of the screen and
// then to the rightmost side, then releases the mouse back at the
// center of the screen. This function can easily incorporate mouse
// drag actions on the screen, by highlighting elements of the page
// with minimal side effects for the test itself. This function works
// best when the center of the screen is a highlightable webpage, such
// as a Google Sheet or a Google Doc.
func RunDragMouseCycle(ctx context.Context, tconn *chrome.TestConn, info *display.Info) error {
	return action.Combine(
		"drag mouse from center of page to the left and right sides and then back to the center",
		mouse.Move(tconn, info.Bounds.CenterPoint(), 500*time.Millisecond),
		mouse.Press(tconn, mouse.LeftButton),
		mouse.Move(tconn, info.Bounds.LeftCenter(), 500*time.Millisecond),
		mouse.Move(tconn, info.Bounds.RightCenter(), time.Second),
		mouse.Move(tconn, info.Bounds.CenterPoint(), 500*time.Millisecond),
		mouse.Release(tconn, mouse.LeftButton),
		action.Sleep(500*time.Millisecond),
	)(ctx)
}
