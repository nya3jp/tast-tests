// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragMaximizedWindowPerf,
		Desc:         "Measures the animation smoothness of dragging a maximized window in clamshell mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
	})
}

func DragMaximizedWindowPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// We are only dragging one window, but have some background windows as occlusion changes can impact performance.
	const numWindows = 5
	conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, numWindows)
	if err != nil {
		s.Fatal("Failed to open browser windows: ", err)
	}
	defer conns.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the internal display info: ", err)
	}
	width := info.WorkArea.Width
	height := info.WorkArea.Height

	// Position the windows so that they will get some occlusion changes while we drag the maximized window. Here we place one in each corner.
	count := 0
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		if _, err := ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventNormal); err != nil {
			return errors.Wrap(err, "failed to set window state")
		}
		bounds := coords.NewRect(0, 0, width, height)
		if count%2 == 0 {
			bounds.Left = width / 2
		}
		if count/2 == 0 {
			bounds.Top = height / 2
		}
		if _, _, err := ash.SetWindowBounds(ctx, tconn, w.ID, bounds, info.ID); err != nil {
			return errors.Wrap(err, "failed to set window bounds")
		}
		count++
		return nil
	}); err != nil {
		s.Fatal("Failed to setup windows: ", err)
	}

	// Maximize the first window. It is required to use this feature.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	window := windows[0]
	if _, err := ash.SetWindowState(ctx, tconn, window.ID, ash.WMEventMaximize); err != nil {
		s.Fatalf("Failed to set the state of window (%d): %v", window.ID, err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	// Check that the window we maximized is the active window, otherwise this test won't work.
	maximizedWindow, err := ash.GetWindow(ctx, tconn, window.ID)
	if err != nil {
		s.Fatal("Failed to obtain window: ", err)
	}
	if !maximizedWindow.IsActive {
		s.Fatal("First window in list is not active window: ", err)
	}

	// Start the drag in the middle of the caption. Drag down to unmaximize, then circle around to trigger some
	// occlusion changes. Finally, drag back to the top to maximize the window again.
	points := []coords.Point{
		// Caption center.
		coords.NewPoint(maximizedWindow.BoundsInRoot.Width/2, 5),
		// Points in some select parts of the display (diamond shape).
		coords.NewPoint(10, height/2),
		coords.NewPoint(width/2, height-10),
		coords.NewPoint(width-10, height/2),
	}
	// Return to the caption center, this will trigger a remaximize animation.
	points = append(points, points[0])

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func() error {
		// Move the mouse to caption and press down.
		if err := mouse.Move(ctx, tconn, points[0], 10*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to move to caption")
		}
		if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
			return errors.Wrap(err, "failed to press the button")
		}

		// Drag the window around.
		const dragTime = 500 * time.Millisecond
		for _, point := range points {
			if err := mouse.Move(ctx, tconn, point, dragTime); err != nil {
				return errors.Wrap(err, "failed to drag")
			}
		}

		// Release the window. It is near the top of the screen so it should snap to maximize.
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			return errors.Wrap(err, "failed to release the button")
		}

		// The window animates when snapping to maximize. Wait for it to finish animating before ending.
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, window.ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}

		// Sanity check to ensure the window is maximized at the end. Otherwise, the test results are not very
		// useful.
		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain the window list")
		}
		if windows[0].State != ash.WindowStateMaximized {
			return errors.New("window is not maximized")
		}
		return nil
	},
		"Ash.Window.AnimationSmoothness.CrossFade.DragMaximize",
		"Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize"),
		perfutil.StoreSmoothness)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
