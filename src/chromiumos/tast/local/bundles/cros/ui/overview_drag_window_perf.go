// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

type dragType int

const (
	dragTypeNormal dragType = iota
	dragTypeSnap
	dragTypeClose
)

type dragFunc func(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter) error

type dragTest struct {
	dt dragType // Type of the drag to run.
	l  string   // Label for the metric name.
	df dragFunc // Function to run the drag test.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewDragWindowPerf,
		Desc:         "Measures the presentation time of window dragging in overview in tablet mode",
		Contacts:     []string{"xiyuan@chromium.org", "mukai@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name: "normal_drag",
			Val: dragTest{
				dt: dragTypeNormal,
				l:  "NormalDrag",
				df: normalDrag,
			},
		}, {
			Name: "drag_to_snap",
			Val: dragTest{
				dt: dragTypeSnap,
				l:  "DragToSnap",
				df: dragToSnap,
			},
		}, {
			Name: "drag_to_close",
			Val: dragTest{
				dt: dragTypeClose,
				l:  "DragToClose",
				df: dragToClose,
			},
		}},
	})
}

func normalDrag(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter) error {
	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return err
	}

	// TODO(crbug.com/1007060): Add API to verify overview drag status.

	// Drag to move it around.
	t := 1 * time.Second
	for _, point := range []struct {
		x, y input.TouchCoord
	}{
		{x + tsw.Width()/2, y},
		{x + tsw.Width()/2, y + tsw.Height()/2},
		{x, y + tsw.Height()/2},
		{x, y},
	} {
		if err := stw.Swipe(ctx, x, y, point.x, point.y, t); err != nil {
			return err
		}

		x = point.x
		y = point.y
	}

	return stw.End()
}

func dragToSnap(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter) error {
	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return err
	}

	// TODO(crbug.com/1007060): Add API to verify overview drag status.

	// Drag to the left edge to snap it.
	if err := stw.Swipe(ctx, x, y, input.TouchCoord(0), y, 500*time.Millisecond); err != nil {
		return err
	}

	if err := stw.End(); err != nil {
		return err
	}

	// TODO(crbug.com/1007060): Add API to verify an item is left snapped.
	return nil
}

func clearSnap(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter) error {
	// Clears snapped window by touch down screen center and move all the way to
	// left.
	x := input.TouchCoord(tsw.Width() / 2)
	y := input.TouchCoord(tsw.Height() / 2)

	if err := stw.Move(x, y); err != nil {
		return err
	}
	if err := stw.Swipe(ctx, x, y, input.TouchCoord(0), y, 500*time.Millisecond); err != nil {
		return err
	}

	// TODO(crbug.com/1007060): Add API to verify no snapped window.
	return stw.End()
}

func dragToClose(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter) error {
	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// No need to long press to pick it up just drag out of screen to close.
	if err := stw.Move(x, y); err != nil {
		return err
	}

	if err := stw.Swipe(ctx, x, y, x, tsw.Height()-1, 500*time.Millisecond); err != nil {
		return err
	}

	if err := stw.End(); err != nil {
		return err
	}

	// Wait for close animation to finish and close the window.
	if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
		return err
	}

	// TODO(crbug.com/1007060): Add API to verify window is really closed.
	return nil
}

func OverviewDragWindowPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)

	if err = ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enable tablet mode: ", err)
	}

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer tsw.Close()
	rotation, err := display.GetPanelRotation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the panel rotation: ", err)
	}
	if err = tsw.SetRotation(-rotation); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	const histName = "Ash.Overview.WindowDrag.PresentationTime.TabletMode"

	hm := map[string]*metrics.Histogram{}
	pv := perf.NewValues()
	drag := s.Param().(dragTest)

	currentWindows := 0
	// Run the test cases with different number of browser windows.
	for _, windows := range []int{2, 8} {
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to open windows: ", err)
		}
		currentWindows = windows
		// Those connections are not used. It's safe to close them now.
		conns.Close()

		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		// Wait for cpu idle after creating windows and entering overview.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		// Run the drag.
		if err := drag.df(ctx, tsw, stw); err != nil {
			s.Fatal("Failed to run drag: ", err)
		}

		// Record the latency metric.
		histogram, err := metrics.UpdateHistogramAndGetDiff(ctx, cr, histName, hm)
		if err != nil {
			s.Fatalf("Failed to get histogram for %s: %v", drag.l, err)
		}
		metricName := fmt.Sprintf("%s.%s.%dwindows", histName, drag.l, currentWindows)
		pv.Set(perf.Metric{
			Name:      metricName,
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, histogram.Mean())
		s.Logf("%s=%f", metricName, histogram.Mean())

		// Clean up.
		switch drag.dt {
		case dragTypeSnap:
			if err := clearSnap(ctx, tsw, stw); err != nil {
				s.Fatal("Failed to clearSnap: ", err)
			}
		case dragTypeClose:
			currentWindows--
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
