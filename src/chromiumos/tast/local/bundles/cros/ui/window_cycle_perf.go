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
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCyclePerf,
		Desc:         "Measures the animation smoothness of window cycle animations when alt + tabbing",
		Contacts:     []string{"yjliu@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func WindowCyclePerf(ctx context.Context, s *testing.State) {
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
	if originalTabletMode {
		ash.SetTabletModeEnabled(ctx, tconn, false)
		defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	numExistingWindows := 0
	prevHists := map[string]*metrics.Histogram{}

	pv := perf.NewValues()
	for _, numWindows := range []int{2, 8} {
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, numWindows-numExistingWindows)
		if err != nil {
			s.Fatal("Failed to open browser windows: ", err)
		}
		conns.Close()

		numExistingWindows = numWindows

		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		// first long press alt + tab to bring up the window cycle view
		if err = keyboard.AccelPress(ctx, "Alt"); err != nil {
			s.Fatal("Failed to press alt: ", err)
		}
		if err = testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			s.Fatal("Failed to wait: ", err)
		}
		if err = keyboard.Accel(ctx, "Tab"); err != nil {
			s.Fatal("Failed to type tab: ", err)
		}

		for i := 0; i < numWindows*2; i++ {
			if err := keyboard.Accel(ctx, "Tab"); err != nil {
				s.Fatal("Failed to type tab: ", err)
			}
			if err = testing.Sleep(ctx, 200*time.Millisecond); err != nil {
				s.Fatal("Failed to wait: ", err)
			}
		}

		if err = testing.Sleep(ctx, 2*time.Second); err != nil {
			s.Fatal("Failed to wait: ", err)
		}

		if err = keyboard.AccelRelease(ctx, "Alt"); err != nil {
			s.Fatal("Failed to release alt: ", err)
		}

		for _, name := range []string{
			"Ash.WindowCycleView.AnimationSmoothness.Show",
			"Ash.WindowCycleView.AnimationSmoothness.Container",
			"Ash.WindowCycleView.AnimationSmoothness.Highlight",
		} {
			histogram, err := metrics.GetHistogram(ctx, cr, name)
			if err != nil {
				s.Fatalf("Failed to get histogram %s: %v", name, err)
			}
			histToReport := histogram
			if prevHist, present := prevHists[name]; present {
				if histToReport, err = histogram.Diff(prevHist); err != nil {
					s.Fatalf("Failed to compute the histogram diff of %s: %v", name, err)
				}
			}

			prevHists[name] = histogram
			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("%s.%dwindows.%dtimes", name, numExistingWindows, histToReport.TotalCount()),
				Unit:      "percent",
				Direction: perf.BiggerIsBetter,
			}, histToReport.Mean())
		}

		if err = pv.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
