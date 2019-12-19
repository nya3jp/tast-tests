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
	"chromiumos/tast/local/chrome/display"
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

	if connected, err := display.PhysicalDisplayConnected(ctx, tconn); err != nil {
		s.Fatal("Failed to check if a physical display is connected or not: ", err)
	} else if !connected {
		s.Log("There are no physical displays and no data can be collected for this test")
		return
	}

	tm, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer tm.Close(ctx)

	pv := perf.NewValues()
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
		if len(ws) != numWindows {
			s.Errorf("The number of windows mismatches: %d vs %d", len(ws), numWindows)
			continue
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
		hists, err := metrics.Run(ctx, cr, func() error {
			if err := ash.MouseDrag(ctx, tconn, start, end, time.Second*2); err != nil {
				return errors.Wrap(err, "failed to drag")
			}
			return nil
		}, "Ash.InteractiveWindowResize.TimeToPresent")
		if err != nil {
			s.Fatal("Failed to drag or get the histogram: ", err)
		}

		latency, err := hists[0].Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", hists[0].Name, err)
		}
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.%dwindows", hists[0].Name, numWindows),
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, latency)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
