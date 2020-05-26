// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragWindowPerf,
		Desc:         "Measures the presentation time of dragging a maximized window in clamshell mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func DragWindowPerf(ctx context.Context, s *testing.State) {
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
	const numWindows = 4
	conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, numWindows)
	if err != nil {
		s.Fatal("Failed to open browser windows: ", err)
	}
	defer conns.Close()

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

	// Start the drag in the middle of the caption. Drag down to unmaximize, then zig zag around to trigger some occlusion changes. Finally, drag back to the top to maximize the window again.
	captionCenter := coords.NewPoint(window.BoundsInRoot.Width/2, 10)
	point1 := coords.NewPoint(0, 300)
	point2 := coords.NewPoint(500, 300)

	hists, err := metrics.Run(ctx, tconn, func() error {
		const dragTime = 500 * time.Millisecond
		if err := ash.MouseDrag(ctx, tconn, captionCenter, point1, dragTime); err != nil {
			return errors.Wrap(err, "failed to drag")
		}
		if err := ash.MouseDrag(ctx, tconn, point1, point2, dragTime); err != nil {
			return errors.Wrap(err, "failed to drag")
		}
		if err := ash.MouseDrag(ctx, tconn, point2, captionCenter, dragTime); err != nil {
			return errors.Wrap(err, "failed to drag")
		}

		// The window animates when snapping to maximize. Wait for it to finish animating before ending.
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, window.ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}
		return nil
	},
		"Ash.Window.AnimationSmoothness.CrossFade")
	if err != nil {
		s.Fatal("Failed to drag or get the histogram: ", err)
	}

	pv := perf.NewValues()
	for _, h := range hists {
		mean, err := h.Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
		}

		pv.Set(perf.Metric{
			Name:      h.Name,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
