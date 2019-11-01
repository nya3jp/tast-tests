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
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowResizePerf,
		Desc:         "Measures animation smoothness of resizing a window",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func WindowResizePerf(ctx context.Context, s *testing.State) {
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

	// Make sure to be in the clamshell mode.
	ash.SetTabletModeEnabled(ctx, tconn, false)

	pv := perf.NewValues()
	const histName = "Ash.InteractiveWindowResize.TimeToPresent"
	for _, numWindows := range []int{1, 2} {
		conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open a new connection: ", err)
		}
		defer conn.Close()

		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil || len(ws) == 0 {
			s.Fatal("Failed to obtain the window list: ", err)
		}
		id0 := ws[0].ID
		if _, err = ash.SetWindowState(ctx, tconn, id0, ash.WMEventSnapLeft); err != nil {
			s.Fatalf("Failed to set the state of window (%d): %v", id0, err)
		}
		if len(ws) > 1 {
			if _, err = ash.SetWindowState(ctx, tconn, ws[1].ID, ash.WMEventSnapRight); err != nil {
				s.Fatalf("Failed to set the state of window (%d): %v", ws[1].ID, err)
			}
		}

		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed to wait: ", err)
		}

		w0, err := ash.GetWindow(ctx, tconn, id0)
		if err != nil {
			s.Error("Failed to get windows: ", err)
		}
		bounds := w0.BoundsInRoot

		start := ash.Location{X: bounds.Left + bounds.Width, Y: bounds.Top + bounds.Height/2}
		if len(ws) > 1 {
			// For multiple windows; hover on the boundary, wait for the resize-handle
			// to appear, and move onto the resize handle.
			if err = ash.MouseMove(ctx, tconn, start, 0); err != nil {
				s.Fatal("Failed to move the mouse: ", err)
			}
			// Waiting for the resize-handle to appear. TODO(mukai): find the right
			// wait to see its visibility.
			if err = testing.Sleep(ctx, 3*time.Second); err != nil {
				s.Fatal("Failed to wait: ", err)
			}
			// 20 DIP would be good enough to move the drag handle.
			start.Y += 20
			if err = ash.MouseMove(ctx, tconn, start, 0); err != nil {
				s.Fatal("Failed to move the mouse: ", err)
			}
		}
		end := ash.Location{X: start.X - bounds.Width/4, Y: start.Y}
		beforeHist, err := metrics.GetHistogram(ctx, cr, histName)
		if err != nil {
			s.Fatalf("Failed to get histogram %s: %v", histName, err)
		}
		if err = ash.MouseDrag(ctx, tconn, start, end, time.Second*2); err != nil {
			s.Fatal("Failed to drag: ", err)
		}

		afterHist, err := metrics.GetHistogram(ctx, cr, histName)
		if err != nil {
			s.Fatalf("Failed to get histogram %s: %v", histName, err)
		}
		diff, err := afterHist.Diff(beforeHist)
		if err != nil {
			s.Fatal("Failed to compute diff: ", err)
		}
		if diff.TotalCount() == 0 {
			s.Fatal("No metric data are found")
		}
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.%dwindows", histName, numWindows),
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, diff.Mean())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
