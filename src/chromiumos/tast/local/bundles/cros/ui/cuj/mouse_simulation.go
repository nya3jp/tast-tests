// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// SimulateMouseMovement moves the mouse in a spiral according to r = theta, starting at the
// center of the screen. The mouse position is reset to the center of the screen once it
// reaches the border of the screen. Using this math function allows for both specifying
// a path for the mouse without defining coordinates in advance, as well as progressively
// increasing the speed that the mouse moves.
func SimulateMouseMovement(ctx context.Context, tconn *chrome.TestConn, duration time.Duration) error {
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
