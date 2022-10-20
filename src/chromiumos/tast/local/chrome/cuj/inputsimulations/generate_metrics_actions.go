// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputsimulations

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
)

// DoAshWorkflows performs a series of system UI interactions
// to generate samples of Ash.Smoothness.PercentDroppedFrames_1sWindow
// (ADF). This function is self contained, in that at the end of the
// function, the state of the device is the same as when DoAshWorkflows
// was first called, except for the location of the mouse. This function
// assumes that when it is called, the device is not in overview mode.
// This function also assumes that at least 1 window is opened at the
// time of the function call.
//
// The action consists of opening and closing the system tray, entering
// overview mode, dragging a window preview around, and exiting
// overview mode.
func DoAshWorkflows(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context) error {
	// Determine the type of |pc|, because we only move the mouse if
	// |pc| is a mouse pointer. This is to ensure we only move the
	// mouse on devices in clamshell mode.
	var touch bool
	switch pc.(type) {
	case nil:
		return errors.New("pointer context is nil")
	case *pointer.MouseContext:
		touch = false
	case *pointer.TouchContext:
		touch = true
	default:
		return errors.New("unrecognized pointer context type")
	}

	// Open and close the system tray bubble.
	systemTray := nodewith.HasClass("UnifiedSystemTray")
	systemTrayContainer := nodewith.HasClass("SystemTrayContainer")
	ac := uiauto.New(tconn)

	if !touch {
		if err := ac.MouseMoveTo(systemTray, 500*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move mouse to the system tray")
		}
	}

	if err := uiauto.Combine(
		"open and close the status tray",
		pc.Click(systemTray),
		ac.WaitUntilExists(systemTrayContainer),
		// Add a fixed sleep to simulate a user looking for the button that
		// they want to press.
		uiauto.Sleep(500*time.Millisecond),
		pc.Click(systemTray),
		ac.WaitUntilGone(systemTrayContainer),
	)(ctx); err != nil {
		return err
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}

	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get overview window")
	}

	dragPoint := w.OverviewInfo.Bounds.CenterPoint()
	if !touch {
		if err := mouse.Move(tconn, dragPoint, time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to move mouse to the first window in overview mode")
		}
	}

	offset := coords.NewPoint(50, 0)
	if err := uiauto.Combine(
		"drag window to the right and left",
		pc.Drag(
			dragPoint,
			pc.DragTo(dragPoint.Add(offset), 500*time.Millisecond),
			pc.DragTo(dragPoint.Sub(offset), time.Second),
			pc.DragTo(dragPoint, 500*time.Millisecond),
		),

		// Sleep to give a fixed amount of time for the preview to
		// stabilize. A fixed value helps keep the overall duration of
		// DoAshWorkflows relatively consistent.
		uiauto.Sleep(time.Second),
	)(ctx); err != nil {
		return err
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to exit overview mode")
	}
	return nil
}
