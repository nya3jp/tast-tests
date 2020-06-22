// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
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

	const (
		pkg          = "com.android.settings"
		activityName = ".Settings"
		dragDur      = time.Second
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

	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set the window state to normal: ", err)
	}

	if err := ash.WaitForCondition(ctx, tconn, func(cur *ash.Window) bool {
		return cur.ID == window.ID && cur.State == ash.WindowStateNormal && !cur.IsAnimating
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to wait for the window to finish animating: ", err)
	}

	// Waits for the window bounds to be updated on the Android side.
	waitForWindowBounds := func(ctx context.Context, expected coords.Rect) error {
		expected = coords.ConvertBoundsFromDpToPx(expected, dispMode.DeviceScaleFactor)
		return testing.Poll(ctx, func(ctx context.Context) error {
			actual, err := act.WindowBounds(ctx)
			if err != nil {
				return testing.PollBreak(err)
			}
			if actual != expected {
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
	initDst := coords.NewPoint(initBounds.Width/2, window.CaptionHeight/2)
	if err := mouse.Move(ctx, tconn, initDst, 0); err != nil {
		s.Fatal("Failed to move the mouse: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()

	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to press the mouse button: ", err)
	}
	defer func(ctx context.Context) {
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			s.Error("Failed to release the mouse button: ", err)
		}
	}(cleanupCtx)

	// Drag the window to the four corners of the work area minus the inset.
	r := info.WorkArea.WithInset(window.CaptionHeight/2, window.CaptionHeight/2)
	for _, dst := range []coords.Point{r.TopLeft(), r.TopRight(), r.BottomRight(), r.BottomLeft()} {
		if err := mouse.Move(ctx, tconn, dst, dragDur); err != nil {
			s.Fatal("Failed to move the mouse: ", err)
		}

		offset := dst.Sub(initDst)
		expectedBounds := initBounds.WithOffset(offset.X, offset.Y)
		if err := waitForWindowBounds(ctx, expectedBounds); err != nil {
			s.Fatal("Failed to wait for the activity to move: ", err)
		}
	}
}
