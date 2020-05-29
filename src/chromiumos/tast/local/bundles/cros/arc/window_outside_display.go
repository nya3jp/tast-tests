// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowOutsideDisplay,
		Desc:         "Ensures an ARC window can move outside the display",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		// TODO(yusukes): Change the timeout back to 4 min when we revert arc.go's BootTimeout to 120s.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func WindowOutsideDisplay(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Could not create a new Display: ", err)
	}
	defer disp.Close()
	sz, err := disp.Size(ctx)
	if err != nil {
		s.Fatal("Failed to get the display size: ", err)
	}

	w, h := sz.Width, sz.Height

	const (
		pkg          = "com.android.settings"
		activityName = ".Settings"
		swipeDur     = time.Second
		marginPX     = 2
	)

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatal("Failed to create the settings activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the settings activity: ", err)
	}
	defer act.Stop(ctx)

	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set the window state to normal: ", err)
	}

	// ResizeWindow and MoveWindow are implemented with touchscreen event injection.
	// Due to its floating point precision, results of these operations are not strictly accurate.
	nearlyEqual := func(a, b int) bool {
		diff := a - b
		if diff < 0 {
			diff = -diff
		}
		return diff <= marginPX
	}

	waitForWindowBounds := func(ctx context.Context, expected coords.Rect) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			actual, err := act.WindowBounds(ctx)
			if err != nil {
				return testing.PollBreak(err)
			}

			if !nearlyEqual(expected.Left, actual.Left) ||
				!nearlyEqual(expected.Top, actual.Top) ||
				!nearlyEqual(expected.Width, actual.Width) ||
				!nearlyEqual(expected.Height, actual.Height) {
				return errors.Errorf("window bounds doesn't match: got %v, want %v", actual, expected)
			}

			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}

	if err := act.MoveWindow(ctx, coords.Point{X: 0, Y: 0}, swipeDur); err != nil {
		s.Fatal("Failed to move the activity: ", err)
	}

	if err := act.ResizeWindow(ctx, arc.BorderBottomRight, coords.Point{X: w / 2, Y: h / 2}, swipeDur); err != nil {
		s.Fatal("Failed to resize the activity: ", err)
	}

	if err := waitForWindowBounds(ctx, coords.Rect{Left: 0, Top: 0, Width: w / 2, Height: h / 2}); err != nil {
		s.Fatal("Failed to wait for the activity to resize: ", err)
	}

	outset := w / 8

	left := -outset
	top := 0
	right := w/2 + outset
	bottom := h/2 + outset

	for _, origin := range []coords.Point{{X: left, Y: top}, {X: right, Y: top}, {X: right, Y: bottom}, {X: left, Y: bottom}} {
		if err := act.MoveWindow(ctx, origin, swipeDur); err != nil {
			s.Fatal("Failed to move the activity: ", err)
		}

		if err := waitForWindowBounds(ctx, coords.Rect{Left: origin.X, Top: origin.Y, Width: w / 2, Height: h / 2}); err != nil {
			s.Fatal("Failed to wait for the activity to move: ", err)
		}
	}
}
