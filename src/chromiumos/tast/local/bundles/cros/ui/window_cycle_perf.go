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

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	numExistingWindows := 0

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

		hists, err := metrics.Run(ctx, tconn, func() error {
			// first long press alt + tab to bring up the window cycle view
			if err := keyboard.AccelPress(ctx, "Alt"); err != nil {
				return errors.Wrap(err, "failed to press alt")
			}
			defer keyboard.AccelRelease(ctx, "Alt")
			if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			if err := keyboard.Accel(ctx, "Tab"); err != nil {
				return errors.Wrap(err, "failed to type tab")
			}

			for i := 0; i < numWindows*2; i++ {
				if err := keyboard.Accel(ctx, "Tab"); err != nil {
					return errors.Wrap(err, "failed to type tab")
				}
				if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
			}

			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			return nil
		}, "Ash.WindowCycleView.AnimationSmoothness.Show",
			"Ash.WindowCycleView.AnimationSmoothness.Container",
		)
		if err != nil {
			s.Fatal("Failed to cycle windows or get the histograms: ", err)
		}
		for _, h := range hists {
			mean, err := h.Mean()
			if err != nil {
				s.Logf("Failed to get mean for histogram %s: %v", h.Name, err)
				continue
			}

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("%s.%dwindows", h.Name, numExistingWindows),
				Unit:      "percent",
				Direction: perf.BiggerIsBetter,
			}, mean)
		}

		if err = pv.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
