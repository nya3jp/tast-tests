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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/pointer"
)

// DragToSnapFirstOverviewWindow finds the first window in overview, and drags to snap it.
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
	target := info.Bounds.RightCenter()
	if primary {
		target = info.Bounds.LeftCenter()
	}

	if err := pc.Drag(center, uiauto.Sleep(2*time.Second), pc.DragTo(target, time.Second))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag to snap from overview")
	}

	return nil
}
