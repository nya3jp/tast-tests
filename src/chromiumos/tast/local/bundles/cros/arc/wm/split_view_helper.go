// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wm provides Window Manager Helper functions.
package wm

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/input"
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
	target := info.Bounds.RightCenter()
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

// ToggleSnapViaKeyboardShortcut snaps (unsnap if it's already snapped) the given activity's window
// to the primary (left/top on the landscape/portrait mode accordingly) side (the secondary side if
// primary is false) by using the keyboard shortcut.
func ToggleSnapViaKeyboardShortcut(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, primary bool) error {
	if err := act.Focus(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to focus the activity")
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer ew.Close()

	shortcutCommand := "Alt+]"
	if primary {
		shortcutCommand = "Alt+["
	}
	if err := ew.Accel(ctx, shortcutCommand); err != nil {
		return errors.Wrap(err, "failed to write keyboard events")
	}

	return nil
}
