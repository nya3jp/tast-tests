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
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragTabInClamshellPerf,
		Desc:         "Measures the presentation time of dragging a tab in clamshell mode",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func DragTabInClamshellPerf(ctx context.Context, s *testing.State) {
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

	conn1, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to open the first tab connection: ", err)
	}
	defer conn1.Close()
	conn2, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to open the second tab connection: ", err)
	}
	defer conn2.Close()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	bounds := ws[0].BoundsInRoot
	// Use a heuristic offset (30, 30) from the window origin for the first tab.
	start := coords.NewPoint(bounds.Left+30, bounds.Top+30)
	end := bounds.CenterPoint()
	hists, err := metrics.Run(ctx, tconn, func() error {
		if err := mouse.Drag(ctx, tconn, start, end, time.Second); err != nil {
			s.Fatal("Failed to drag to the end point: ", err)
		}
		// Expecting 2 windows.
		if err := WindowsNumCheck(ctx, tconn, 2); err != nil {
			s.Fatal("Wrong number of windows: ", err)
		}
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}
		if err := mouse.Drag(ctx, tconn, end, start, time.Second); err != nil {
			s.Fatal(err, "Failed to drag back to the start point: ", err)
		}
		// Expecting 1 window.
		if err := WindowsNumCheck(ctx, tconn, 1); err != nil {
			s.Fatal("Wrong number of windows: ", err)
		}
		return nil
	},
		"Ash.WorkspaceWindowResizer.TabDragging.PresentationTime.ClamshellMode",
		"Ash.WorkspaceWindowResizer.TabDragging.PresentationTime.MaxLatency.ClamshellMode")
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
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, mean)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

func WindowsNumCheck(ctx context.Context, tconn *chrome.TestConn, num int) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrapf(err, "failed to obtain the window list")
	}
	if num != len(ws) {
		return errors.Wrapf(err, "expected %v windows, got %v windows", num, len(ws))
	}
	return nil
}
