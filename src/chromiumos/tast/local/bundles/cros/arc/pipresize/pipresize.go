// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pipresize provides functionality for controlling the size of an
// ARC++ PIP window.
package pipresize

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// WaitForPIPAndSetSize waits for a PIP window to appear, and then ensures
// the requested size, assuming that it is initially as small as possible.
// big=false means keep that size. big=true means make it as big as possible.
func WaitForPIPAndSetSize(ctx context.Context, tconn *chrome.TestConn, d *ui.Device, big bool) error {
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	displayMode, err := info.GetSelectedMode()
	if err != nil {
		return errors.Wrap(err, "failed to get the selected display mode of the primary display")
	}

	var pipWindow *ash.Window
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		pipWindow, err = ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.State == ash.WindowStatePIP })
		if err != nil {
			return errors.Wrap(err, "the PIP window hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for PIP window")
	}

	if !big {
		if 10*pipWindow.TargetBounds.Width >= 3*info.WorkArea.Width && 10*pipWindow.TargetBounds.Height >= 3*info.WorkArea.Height {
			return errors.Errorf("expected small PIP window. Got a %v PIP window in a %v work area", pipWindow.TargetBounds.Size(), info.WorkArea.Size())
		}
		return nil
	}

	if err := mouse.Move(ctx, tconn, pipWindow.TargetBounds.CenterPoint(), time.Second); err != nil {
		return errors.Wrap(err, "failed to move mouse to PIP window")
	}

	// The PIP resize handle is an ImageView with no android:contentDescription.
	// Here we use the regex (?!.+) to match the empty content description. See:
	// frameworks/base/packages/SystemUI/res/layout/pip_menu_activity.xml
	resizeHandleBounds, err := d.Object(
		ui.ClassName("android.widget.ImageView"),
		ui.DescriptionMatches("(?!.+)"),
		ui.PackageName("com.android.systemui"),
	).GetBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get bounds of PIP resize handle")
	}

	if err := mouse.Move(ctx, tconn, coords.ConvertBoundsFromPXToDP(resizeHandleBounds, displayMode.DeviceScaleFactor).CenterPoint(), time.Second); err != nil {
		return errors.Wrap(err, "failed to move mouse to PIP resize handle")
	}
	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to press left mouse button")
	}
	if err := mouse.Move(ctx, tconn, info.WorkArea.TopLeft(), time.Second); err != nil {
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			return errors.Wrap(err, "failed to move mouse for dragging PIP resize handle, and then failed to release left mouse button")
		}
		return errors.Wrap(err, "failed to move mouse for dragging PIP resize handle")
	}
	if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to release left mouse button")
	}

	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location-change events to be propagated to the automation API")
	}

	pipWindow, err = ash.GetWindow(ctx, tconn, pipWindow.ID)
	if err != nil {
		return errors.Wrap(err, "PIP window gone after resize")
	}

	if 5*pipWindow.TargetBounds.Width <= 2*info.WorkArea.Width && 5*pipWindow.TargetBounds.Height <= 2*info.WorkArea.Height {
		return errors.Errorf("expected big PIP window. Got a %v PIP window in a %v work area", pipWindow.TargetBounds.Size(), info.WorkArea.Size())
	}

	return nil
}
