// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragMaximizedWindowPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the animation smoothness of dragging a maximized window in clamshell mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(
			hwdep.InternalDisplay(),
			hwdep.SkipOnModel("burnet"),
		),
		Params: []testing.Param{
			{
				Fixture: "chromeLoggedIn",
				Val:     browser.TypeAsh,
			},
			{
				Name:              "lacros",
				Fixture:           "lacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               browser.TypeLacros,
			},
		},
	})
}

func DragMaximizedWindowPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// We are only dragging one window, but have some (4) background windows as occlusion changes can impact performance.
	const numWindows = 5
	const url = ui.PerftestURL
	// Open a first browser.
	conn, br, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), url)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)
	defer conn.Close()
	if err := ash.CreateWindows(ctx, tconn, br, url, numWindows-1); err != nil {
		s.Fatal("Failed to open browser windows: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the internal display info: ", err)
	}
	width := info.WorkArea.Width
	height := info.WorkArea.Height

	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized); err != nil {
			return errors.Wrapf(err, "failed to maximize window %d", w.ID)
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to setup windows: ", err)
	}

	// Get the first window.
	maximizedWindow, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
		return true
	})
	if err != nil {
		s.Fatal("Failed to obtain the first window: ", err)
	}

	if maximizedWindow.State != ash.WindowStateMaximized {
		s.Fatalf("The first window %d is not maximized (state %q)", maximizedWindow.ID, maximizedWindow.State)
	}
	if !maximizedWindow.IsActive {
		s.Fatalf("The first window %d is not active", maximizedWindow.ID)
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

	pv := perfutil.RunMultiple(ctx, br, uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Move the mouse to caption and press down.
		if err := mouse.Move(tconn, points[0], 10*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to caption")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the button")
		}

		// Drag the window around.
		const dragTime = 500 * time.Millisecond
		for _, point := range points {
			if err := mouse.Move(tconn, point, dragTime)(ctx); err != nil {
				return errors.Wrap(err, "failed to drag")
			}
		}

		// Needs to wait a bit before releasing the mouse, otherwise the window
		// may not get back to be maximized.  See https://crbug.com/1158548.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait")
		}

		// Release the window. It is near the top of the screen so it should snap to maximize.
		if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to release the button")
		}

		// The window animates when snapping to maximize. Wait for it to finish animating before ending.
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, maximizedWindow.ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}

		// Validity check to ensure the window is maximized at the end. Otherwise, the test results are not very
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
		"Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize")),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
