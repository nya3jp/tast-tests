// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputsimulations

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
)

// RunAshUIAction performs a series of system UI interactions
// to generate samples of Ash.Smoothness.PercentDroppedFrames_1sWindow
// (ADF). This function is self contained, in that at the end of the
// function, the state of the device is the same as when it was first
// called, except for the location of the mouse. The device cannot be
// in overview mode when this function is called.
// The action consists of opening and closing the system tray, entering
// overview mode, dragging a window preview around, and exiting
// overview mode.
func RunAshUIAction(ctx context.Context, tconn *chrome.TestConn, pc pointer.Context) error {
	// Open and close the system tray bubble.
	systemTray := nodewith.HasClass("UnifiedSystemTray")
	systemTrayContainer := nodewith.HasClass("SystemTrayContainer")
	ac := uiauto.New(tconn)
	if err := action.Combine(
		"open the status tray",
		ac.MouseMoveTo(systemTray, 500*time.Millisecond),
		ac.LeftClick(systemTray),
		ac.WaitUntilExists(systemTrayContainer),
		// Add a fixed sleep to simulate a user trying to find the button
		// that they want to press.
		action.Sleep(500*time.Millisecond),
		ac.LeftClick(systemTray),
		ac.WaitUntilGone(systemTrayContainer),
	)(ctx); err != nil {
		return err
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to enter overview mode")
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get active window")
	}
	if len(ws) == 0 {
		return errors.Wrap(err, "failed to find any windows in overview mode")
	}
	w := ws[0]

	dragPoint := w.OverviewInfo.Bounds.CenterPoint()
	offset := coords.NewPoint(50, 0)
	if err := uiauto.Combine(
		"drag window to the right and left",
		mouse.Move(tconn, dragPoint, time.Second),
		pc.Drag(
			dragPoint,
			pc.DragTo(dragPoint.Add(offset), 500*time.Millisecond),
			pc.DragTo(dragPoint.Sub(offset), time.Second),
			pc.DragTo(dragPoint, 500*time.Millisecond),
		),

		// Sleep to give a fixed amount of time for the preview to
		// stabilize. A fixed value helps keep the overall duration of
		// this function relatively consistent.
		action.Sleep(time.Second),
	)(ctx); err != nil {
		return err
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to exit overview mode")
	}
	return nil
}
