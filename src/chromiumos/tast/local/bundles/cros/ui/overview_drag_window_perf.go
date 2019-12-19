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
	_, err := ash.DraggedWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get dragged overview item")
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

// leftOrTop returns the coordinates of left or top position based on screen
// orientation and the given coordinates.
func leftOrTop(ctx context.Context, tconn *chrome.Conn, x, y input.TouchCoord) (input.TouchCoord, input.TouchCoord, error) {
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return x, y, errors.Wrap(err, "failed to get screen orientation")
	}
	var tx, ty input.TouchCoord
	switch orientation.Type {
	case display.OrientationLandscapePrimary:
		tx = input.TouchCoord(0)
		ty = y
	case display.OrientationPortraitPrimary:
		tx = x
		ty = input.TouchCoord(0)
	default:
		return x, y, errors.New("unsupported screen orientation")
	}

	return tx, ty, nil
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
	_, err := ash.DraggedWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get dragged overview item")
	}

	// Get left or top target position to drag to.
	tx, ty, err := leftOrTop(ctx, tconn, x, y)
	if err != nil {
		return err
	}

	// Drag to the left or top edge to snap it.
	if err := stw.Swipe(ctx, x, y, tx, ty, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release touch")
	}

	// Sanity check to ensure one left snapped window.
	snapped, err := ash.SnappedWindows(ctx, tconn)
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
	snapped, err := ash.SnappedWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get snapped windows")
	}
	if len(snapped) == 0 {
		// Do nothing when there is no snapped window.
		return nil
	}

	// Touch down the work area center on the split view divider.
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	pixelToTouchFactorX := float64(tsw.Width()) / float64(info.Bounds.Width)
	pixelToTouchFactorY := float64(tsw.Height()) / float64(info.Bounds.Height)

	x := input.TouchCoord(float64(info.WorkArea.Width) * pixelToTouchFactorX / 2)
	y := input.TouchCoord(float64(info.WorkArea.Height) * pixelToTouchFactorY / 2)

	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move touch")
	}

	// Drag to left or top edge to clear the snap.
	tx, ty, err := leftOrTop(ctx, tconn, x, y)
	if err != nil {
		return err
	}

	if err := stw.Swipe(ctx, x, y, tx, ty, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release touch")
	}

	// Sanity check that there is no longer snapped windows.
	snapped, err = ash.SnappedWindows(ctx, tconn)
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
		return errors.Errorf("window is not closed, got:%d, expect:%d",
			len(newWindows), len(oldWindows)-1)
	}
	return nil
}

func OverviewDragWindowPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	tm, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer tm.Close(ctx)

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer tsw.Close()
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the display orientation: ", err)
	}
	if err = tsw.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	const histName = "Ash.Overview.WindowDrag.PresentationTime.TabletMode"

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

		// Run the drag and collect histogram.
		histograms, err := metrics.Run(ctx, cr, func() error {
			if err := drag.df(ctx, tsw, stw, tconn); err != nil {
				return errors.Wrap(err, "failed to run drag")
			}
			return nil
		}, histName)
		if err != nil {
			s.Fatalf("Failed to drag or get histogram %v: %v", histName, err)
		}

		// Record the latency metric.
		latency, err := histograms[0].Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", histograms[0].Name, err)
		}
		metricName := fmt.Sprintf("%s.%s.%dwindows", histograms[0].Name, drag.l, currentWindows)
		pv.Set(perf.Metric{
			Name:      metricName,
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, latency)
		s.Logf("%s=%f", metricName, latency)

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
