// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/cdputil"
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
	stw *input.SingleTouchEventWriter, tconn *chrome.Conn) error

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
	stw *input.SingleTouchEventWriter, tconn *chrome.Conn) error {
	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return errors.Wrap(err, "failed to long press")
	}

	// Sanity check to ensure there is one dragging item.
	w, err := ash.GetDraggedWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get dragged overview item")
	}
	if w == nil {
		return errors.New("no dragged overview item")
	}

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
			return errors.Wrap(err, "failed to swipe")
		}

		x = point.x
		y = point.y
	}

	return stw.End()
}

func dragToSnap(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter, tconn *chrome.Conn) error {
	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return errors.Wrap(err, "failed to long press")
	}

	// Sanity check to ensure there is one dragging item.
	w, err := ash.GetDraggedWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get dragged overview item")
	}
	if w == nil {
		return errors.New("no dragged overview item")
	}

	// Drag to the left edge to snap it.
	if err := stw.Swipe(ctx, x, y, input.TouchCoord(0), y, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release touch")
	}

	// Sanity check to ensure one left snapped window.
	snapped, err := ash.GetSnappedWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get snapped windows")
	}
	if len(snapped) != 1 || snapped[0].State != ash.WindowStateLeftSnapped {
		return errors.New("left snapped window not found")
	}
	return nil
}

func clearSnap(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter, tconn *chrome.Conn) error {
	// Checks whether there is a snapped window.
	snapped, err := ash.GetSnappedWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get snapped windows")
	}
	if len(snapped) == 0 {
		// Do nothing when there is no snapped window.
		return nil
	}

	// Clears snapped window by touch down screen center and move all the way to
	// left.
	x := input.TouchCoord(tsw.Width() / 2)
	y := input.TouchCoord(tsw.Height() / 2)

	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move touch")
	}
	if err := stw.Swipe(ctx, x, y, input.TouchCoord(0), y, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release touch")
	}

	// Sanity check that there is no longer snapped windows.
	snapped, err = ash.GetSnappedWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get snapped windows")
	}
	if len(snapped) != 0 {
		return errors.New("failed to clear snapped window")
	}

	return nil
}

func dragToClose(ctx context.Context, tsw *input.TouchscreenEventWriter,
	stw *input.SingleTouchEventWriter, tconn *chrome.Conn) error {
	// Get existing windows before closing.
	oldWindows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all windows")
	}

	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// No need to long press to pick it up just drag out of screen to close.
	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move touch")
	}

	if err := stw.Swipe(ctx, x, y, x, tsw.Height()-1, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release touch")
	}

	// Wait for close animation to finish and close the window.
	if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	// Sanity check that a window is closed.
	newWindows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all windows")
	}
	if len(oldWindows) != len(newWindows)+1 {
		return errors.Errorf("Window is not closed, before:%d, after:%d",
			len(oldWindows), len(newWindows))
	}
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
		for ; currentWindows < windows; currentWindows++ {
			conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
			if err != nil {
				s.Fatal("Failed to open a new connection: ", err)
			}
			defer conn.Close()
		}

		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		// Wait for cpu idle after creating windows and entering overview.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		// Run the drag.
		if err := drag.df(ctx, tsw, stw, tconn); err != nil {
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
			if err := clearSnap(ctx, tsw, stw, tconn); err != nil {
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
