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
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewDragWindowPerf,
		Desc:         "Measures the presentation time of window dragging in overview in tablet mode",
		Contacts:     []string{"xiyuan@chromium.org", "mukai@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
		Timeout:      6 * time.Minute,
	})
}

// recordLatency records the mean of histogram |histName| diffs of a latency
// under |metricName|.
func recordLatency(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	histName string, hm map[string]*metrics.Histogram, metricName string,
	pv *perf.Values) error {
	histogram, err := metrics.UpdateHistogramAndGetDiff(ctx, cr, histName, hm)
	if err != nil {
		return err
	}

	pv.Set(perf.Metric{
		Name:      metricName,
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, histogram.Mean())
	s.Logf("%s=%f", metricName, histogram.Mean())
	return nil
}

func normalDrag(ctx context.Context, s *testing.State,
	tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter) error {
	x := input.TouchCoord(tsw.Width() / 3)
	y := input.TouchCoord(tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return err
	}

	// TODO(crbug.com/1007060): Add API to verify overview drag status.

	// Drag to move it around.
	t := 1000 * time.Millisecond
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

func dragToSnap(ctx context.Context, s *testing.State,
	tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter) error {
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

func clearSnap(ctx context.Context, s *testing.State,
	tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter) error {
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

func dragToClose(ctx context.Context, s *testing.State,
	tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter) error {
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

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	const histName = "Ash.Overview.WindowDrag.PresentationTime.TabletMode"

	type dragType int
	const (
		dragTypeNormal dragType = iota
		dragTypeSnap
		dragTypeClose
	)

	prevHists := map[string]*metrics.Histogram{}
	pv := perf.NewValues()

	currentWindows := 0
	// Run the test cases in different number of browser windows.
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

		type runDragFunc func(ctx context.Context, s *testing.State,
			tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter) error
		for _, run := range []struct {
			dt    dragType
			fn    runDragFunc
			label string
		}{
			{dt: dragTypeNormal, fn: normalDrag, label: "NormalDrag"},
			{dt: dragTypeSnap, fn: dragToSnap, label: "DragToSnap"},
			{dt: dragTypeClose, fn: dragToClose, label: "DragToClose"},
		} {
			if err := cpu.WaitUntilIdle(ctx); err != nil {
				s.Fatal("Failed waiting for CPU to become idle: ", err)
			}
			if err := run.fn(ctx, s, tsw, stw); err != nil {
				s.Fatalf("Failed run %s: %v", run.label, err)
			}
			if err := recordLatency(ctx, s, cr, histName, prevHists,
				fmt.Sprintf("%s.%s.%dwindows", histName, run.label, currentWindows),
				pv); err != nil {
				s.Fatalf("Failed to record latency for %s: %v", run.label, err)
			}

			switch run.dt {
			case dragTypeSnap:
				if err := clearSnap(ctx, s, tsw, stw); err != nil {
					s.Fatal("Failed to clearSnap: ", err)
				}
			case dragTypeClose:
				currentWindows--
			}
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
