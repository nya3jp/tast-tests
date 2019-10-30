// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"

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
		Func:         OverviewPerf,
		Desc:         "Measures animation smoothness of entering/exiting the overview mode",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      2 * time.Minute,
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

	currentWindows := 0
	// Run the overview mode enter/exit flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode with maximized windows or
	//   tablet mode. TODO(mukai): add clamshell mode with normal windows.
	for _, windows := range []int{2, 8} {
		for ; currentWindows < windows; currentWindows++ {
			conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
			if err != nil {
				s.Fatal("Failed to open a new connection: ", err)
			}
			defer conn.Close()
		}

		for _, inTabletMode := range []bool{false, true} {
			if err = ash.SetTabletModeEnabled(ctx, tconn, inTabletMode); err != nil {
				s.Fatalf("Failed to set tablet mode %v: %v", inTabletMode, err)
			}

			if err = cpu.WaitUntilIdle(ctx); err != nil {
				s.Fatal("Failed to wait for system UI to be stabilized: ", err)
			}

			if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}
			if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
				s.Fatal("Failed to exit from the overview mode: ", err)
			}

			suffix := "SingleClamshellMode"
			if inTabletMode {
				suffix = "TabletMode"
			}

			for _, prefix := range []string{
				"Ash.Overview.AnimationSmoothness.Enter",
				"Ash.Overview.AnimationSmoothness.Exit",
			} {
				histName := prefix + "." + suffix
				histogram, err := metrics.GetHistogram(ctx, cr, histName)
				if err != nil {
					s.Fatalf("Failed to get histogram %s: %v", histName, err)
				}
				pv.Set(perf.Metric{
					Name:      fmt.Sprintf("%s.%dwindows", histName, currentWindows),
					Unit:      "percent",
					Direction: perf.BiggerIsBetter,
				}, histogram.Mean())
			}
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
