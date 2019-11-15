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
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewPerf,
		Desc:         "Measures animation smoothness of entering/exiting the overview mode",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func OverviewPerf(ctx context.Context, s *testing.State) {
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

	pv := perf.NewValues()

	// overviewAnimationType specifies the type of the animation of entering to or
	// exiting from the overview mode.
	type overviewAnimationType int
	const (
		// animationTypeMaximized is the animation when there are maximized windows
		// in the clamshell mode.
		animationTypeMaximized overviewAnimationType = iota
		// animationTypeNormalWindow is the animation for normal windows in the
		// clamshell mode.
		animationTypeNormalWindow
		// animationTypeTabletMode is the animation for windows in the tablet mode.
		animationTypeTabletMode
	)

	currentWindows := 0
	// Run the overview mode enter/exit flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode with maximized windows or
	//   tablet mode.
	for _, windows := range []int{2, 8} {
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		defer conns.Close()
		currentWindows = windows

		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system UI to be stabilized: ", err)
		}

		for _, state := range []overviewAnimationType{animationTypeMaximized, animationTypeNormalWindow, animationTypeTabletMode} {
			inTabletMode := state == animationTypeTabletMode
			if err = ash.SetTabletModeEnabled(ctx, tconn, inTabletMode); err != nil {
				s.Fatalf("Failed to set tablet mode %v: %v", inTabletMode, err)
			}

			if !inTabletMode {
				eventType := ash.WMEventNormal
				if state == animationTypeMaximized {
					eventType = ash.WMEventMaximize
				}
				ws, err := ash.GetAllWindows(ctx, tconn)
				if err != nil {
					s.Fatal("Failed to obtain the window list: ", err)
				}
				for _, w := range ws {
					if _, err := ash.SetWindowState(ctx, tconn, w.ID, eventType); err != nil {
						s.Fatalf("Failed to set the window (%d): %v", w.ID, err)
					}
				}
			}

			// Wait for 3 seconds to stabilize the result. Note that this doesn't
			// have to be cpu.WaitUntilIdle(). It may wait too much.
			// TODO(mukai): find the way to wait more properly on the idleness of Ash.
			// https://crbug.com/1001314.
			if err = testing.Sleep(ctx, 3*time.Second); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			var suffix string
			switch state {
			case animationTypeMaximized:
				suffix = "SingleClamshellMode"
			case animationTypeNormalWindow:
				suffix = "ClamshellMode"
			case animationTypeTabletMode:
				suffix = "TabletMode"
			}

			names := []string{
				"Ash.Overview.AnimationSmoothness.Enter" + "." + suffix,
				"Ash.Overview.AnimationSmoothness.Exit" + "." + suffix,
			}
			histograms, err := metrics.Run(ctx, cr, func() error {
				if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
					return errors.Wrap(err, "failed to enter into the overview mode")
				}
				if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
					return errors.Wrap(err, "failed to exit from the overview mode")
				}
				return nil
			}, names...)
			if err != nil {
				s.Fatal("Failed to enter/exit overview mode or get histograms: ", err)
			}

			for _, h := range histograms {
				pv.Set(perf.Metric{
					Name:      fmt.Sprintf("%s.%dwindows", h.Name, currentWindows),
					Unit:      "percent",
					Direction: perf.BiggerIsBetter,
				}, h.Mean())
			}
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
