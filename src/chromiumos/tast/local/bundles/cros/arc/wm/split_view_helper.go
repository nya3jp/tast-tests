// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wm provides Window Manager Helper functions.
package wm

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// DragToSnapFirstOverviewWindow finds the first window in overview, and drags
// to snap it to the primary side (the secondary side if primary is false) by
// using the given pointer.Context.
// This function assumes that overview is already active.
func DragToSnapFirstOverviewWindow(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context, primary bool) error {
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		// If you see this error on the second window snap (to the right), check if
		// b/143499564 has been reintroduced.
		return errors.Wrap(err, "failed to find window in overview grid")
	}

	center := w.OverviewInfo.Bounds.CenterPoint()
	// The possible x coord on a touchscreen is [0, info.Bounds.Width).
	target := info.Bounds.RightCenter().Sub(coords.NewPoint(1, 0))
	if primary {
		target = info.Bounds.LeftCenter()
	}

	waitForDragged := func(ctx context.Context) error {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := ash.DraggedWindowInOverview(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to get dragged overview item")
			}
			return nil
		}, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for overview item to be dragged")
		}
		return nil
	}

	if err := pc.Drag(center, waitForDragged, pc.DragTo(target, time.Second))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag to snap from overview")
	}

	return nil
}
