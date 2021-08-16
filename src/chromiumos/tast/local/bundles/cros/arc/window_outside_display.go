// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowOutsideDisplay,
		Desc:         "Ensures an ARC window can move outside the display",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedInClamshellMode",
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func WindowOutsideDisplay(ctx context.Context, s *testing.State) {
	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	const (
		pkg          = "com.android.settings"
		activityName = ".Settings"
		dragDur      = time.Second
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
	defer act.Stop(ctx, tconn)

	window, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		return window.ARCPackageName == pkg
	})
	if err != nil {
		s.Fatal("Failed to find the ARC window: ", err)
	}
	info, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool {
		return info.ID == window.DisplayID
	})
	if err != nil {
		s.Fatal("Failed to find the display: ", err)
	}

	dispMode, err := info.GetSelectedMode()
	if err != nil {
		s.Fatal("Failed to get the selected display mode: ", err)
	}

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set the window state to normal: ", err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(cur *ash.Window) bool {
		return cur.ID == window.ID && cur.State == ash.WindowStateNormal && !cur.IsAnimating
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to wait for the window to finish animating: ", err)
	}

	nearlyEqual := func(a, b int) bool {
		diff := a - b
		if diff < 0 {
			diff = -diff
		}
		return diff <= marginPX
	}

	// Waits for the window bounds to be updated on the Android side.
	waitForWindowBounds := func(ctx context.Context, expected coords.Rect) error {
		expected = coords.ConvertBoundsFromDPToPX(expected, dispMode.DeviceScaleFactor)
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

	// Use Bounds instead of WorkArea as it is always divisible by 4.
	initBounds := info.Bounds
	// Use large enough bounds as the Settings activity has minimum height / width.
	initBounds.Width = initBounds.Width / 4 * 3
	initBounds.Height = initBounds.Height / 4 * 3
	if actualBounds, _, err := ash.SetWindowBounds(ctx, tconn, window.ID, initBounds, window.DisplayID); err != nil {
		s.Fatal("Failed to set window bounds: ", err)
	} else if actualBounds != initBounds {
		s.Fatalf("Failed to resize the activity: got %v; want %v", actualBounds, initBounds)
	}

	// Grab the center of the window caption bar.
	initPoint := coords.NewPoint(initBounds.Width/2, window.CaptionHeight/2)

	// If the drag point gets closer to an edge than this value, it will be snapped on drop.
	// As we don't want snapping, we should inset this value.
	// The value is should be in sync with ash/wm/workspace/workspace_window_resizer.cc.
	const ScreenEdgeInsetForSnappingSides = 32

	// Drag the window to the four corners of the work area minus the inset.
	r := info.WorkArea.WithInset(ScreenEdgeInsetForSnappingSides+marginPX, window.CaptionHeight/2)
	src := initPoint
	for _, dst := range []coords.Point{r.TopLeft(), r.TopRight(), r.BottomRight(), r.BottomLeft()} {
		if err := mouse.Drag(ctx, tconn, src, dst, dragDur); err != nil {
			s.Fatal("Failed to move the mouse: ", err)
		}
		src = dst

		offset := dst.Sub(initPoint)
		expectedBounds := initBounds.WithOffset(offset.X, offset.Y)
		if err := waitForWindowBounds(ctx, expectedBounds); err != nil {
			s.Fatal("Failed to wait for the activity to move: ", err)
		}
	}
}
