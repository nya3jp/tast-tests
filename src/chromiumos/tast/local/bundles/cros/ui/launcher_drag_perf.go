// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherDragPerf,
		Desc:         "Measures animation smoothness of lancher animations",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          ash.LoggedInWith100DummyApps(),
		Timeout:      3 * time.Minute,
	})
}

func LauncherDragPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if connected, err := display.PhysicalDisplayConnected(ctx, tconn); err != nil {
		s.Fatal("Failed to check physical display existence: ", err)
	} else if !connected {
		s.Log("Can't collect data points unless there's a physical display")
		return
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	var primaryBounds *display.Bounds
	var primaryWorkArea *display.Bounds
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the display info: ", err)
	}
	for _, info := range infos {
		if info.IsPrimary {
			primaryBounds = info.Bounds
			primaryWorkArea = info.WorkArea
			break
		}
	}
	if primaryBounds == nil {
		s.Fatal("No primary display is found")
	}

	// Points for dragging to open/close the launchers.
	// - X position: about 1/4 of the screen to point to the background of
	//   the shelf (the center would fall into the chrome icon; there are other
	//   buttons on the left/right edge).
	// - Y of bottom: in the middle of shelf (bottom of workarea + half of the
	//   shelf, which equals to the average of workarea height and display height)
	// - Y of top: 10px from the top of the screen; this is just like almost top
	//   of the screen.
	bottom := ash.Location{X: primaryBounds.Left + primaryBounds.Width/4, Y: primaryBounds.Top + (primaryBounds.Height+primaryWorkArea.Height)/2}
	top := ash.Location{X: bottom.X, Y: primaryBounds.Top + 10}

	pv := perf.NewValues()
	currentWindows := 0
	// Run the dragging gesture for different numbers of browser windows (0 or 2).
	for _, windows := range []int{0, 2} {
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		if err := conns.Close(); err != nil {
			s.Error("Failed to close the connection to chrome")
		}
		currentWindows = windows
		// The best effort to stabilize CPU usage. This may or
		// may not be satisfied in time.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system UI to be stabilized: ", err)
		}

		histograms, err := metrics.Run(ctx, cr, func() error {
			// Drag from the bottom to the top; this should expand the app-list to
			// fullscreen.
			if err := ash.MouseDrag(ctx, tconn, bottom, top, time.Second); err != nil {
				return errors.Wrap(err, "failed to drag from the bottom to top")
			}
			if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
				return errors.Wrap(err, "failed to switch the state to 'FullscreenAllApps'")
			}
			// Drag from the top to the bottom; this should close the app-list.
			if err := ash.MouseDrag(ctx, tconn, top, bottom, time.Second); err != nil {
				return errors.Wrap(err, "failed to drag from the top to bottom")
			}
			if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
				return errors.Wrap(err, "failed to switch the state to 'Closed'")
			}
			return nil
		},
			"Apps.StateTransition.Drag.PresentationTime.ClamshellMode")
		if err != nil {
			s.Fatal("Failed to run luancher animation or get histograms: ", err)
		}

		for _, h := range histograms {
			mean, err := h.Mean()
			if err != nil {
				s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
			}

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("%s.%dwindows", h.Name, currentWindows),
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, mean)
		}
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
