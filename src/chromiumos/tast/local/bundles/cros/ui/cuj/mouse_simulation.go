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
)

// SimulateMouseMovement moves the mouse in spiral according to r = theta, starting at the
// center of the screen. The mouse position is reset to the center of the screen once it
// reaches the border of the screen. Using this math function allows for both specifying
// a path for the mouse without defining coordinates in advance, as well as progressively
// increasing the speed that the mouse moves.
func SimulateMouseMovement(ctx context.Context, tconn *chrome.TestConn, duration time.Duration) error {
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	// Move mouse to the center of the screen.
	center := info.WorkArea.CenterPoint()
	mouse.Move(tconn, info.WorkArea.CenterPoint(), 0)

	const deltaTheta = math.Pi / 12
	const deltaTime = 100 * time.Millisecond
	theta := 0.0
	endTime := time.Now().Add(duration)
	for endTime.Sub(time.Now()).Seconds() > 0 {
		theta += deltaTheta
		point := coords.NewPoint(
			center.X+int(theta*math.Cos(theta)),
			center.Y+int(theta*math.Sin(theta)))
		if !point.In(info.WorkArea) {
			point = center
			theta = 0.0
		}
		if err := mouse.Move(tconn, point, deltaTime)(ctx); err != nil {
			return errors.Wrap(err, "failed to move mouse")
		}
	}
	return nil
}
