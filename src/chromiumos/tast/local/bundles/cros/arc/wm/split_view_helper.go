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
	"chromiumos/tast/local/coords"
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

func windowDragPoint(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity) (coords.Point, error) {
	window, err := ash.GetARCAppWindowInfo(ctx, tconn, act.PackageName())
	if err != nil {
		return coords.NewPoint(0, 0), errors.Wrap(err, "failed to get window info")
	}
	// As resize-locked windows have the compat mode button at the center of the caption, we need to drag on the right of the back button instead of the center point.
	return coords.NewPoint(window.BoundsInRoot.Left+100, window.BoundsInRoot.Top+window.CaptionHeight/2), nil
}

// DragCaptionToSnap drags the given activity's caption to snap it to the primary side (the secondary
// side if primary is false) by using the given pointer.Context.
func DragCaptionToSnap(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context, displayInfo *display.Info, act *arc.Activity, primary bool) error {
	if err := act.Focus(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to focus the activity")
	}

	snapDestinationX := displayInfo.Bounds.Width
	if primary {
		snapDestinationX = 0
	}
	dragPoint, err := windowDragPoint(ctx, tconn, act)
	if err != nil {
		return errors.Wrap(err, "failed to get window drag point")
	}
	if err := pc.Drag(
		dragPoint,
		pc.DragTo(coords.NewPoint(snapDestinationX, displayInfo.Bounds.Height/2), 2*time.Second))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag to snap")
	}

	return nil
}

// DragCaptionToUnsnap drags the given activity's caption to unsnap it by using the given
// pointer.Context.
func DragCaptionToUnsnap(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context, displayInfo *display.Info, act *arc.Activity) error {
	if err := act.Focus(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to focus the activity")
	}

	dragPoint, err := windowDragPoint(ctx, tconn, act)
	if err != nil {
		return errors.Wrap(err, "failed to get window drag point")
	}
	if err := pc.Drag(
		dragPoint,
		pc.DragTo(displayInfo.Bounds.CenterPoint(), 2*time.Second))(ctx); err != nil {
		return errors.Wrap(err, "failed to drag to unsnap")
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
