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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRotationPerf,
		Desc:         "Measures animation smoothness of screen rotation in tablet mode",
		Contacts:     []string{"chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
	})
}

func ScreenRotationPerf(ctx context.Context, s *testing.State) {
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

	// Enter tablet mode.
	if err = ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enable tablet mode: ", err)
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	pv := perf.NewValues()
	currentWindows := 0
	prevHists := map[string]*metrics.Histogram{}
	// Run the screen rotation in overview mode with 2 or 8 windows.
	for _, windows := range []int{2, 8} {
		for ; currentWindows < windows; currentWindows++ {
			conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
			if err != nil {
				s.Fatal("Failed to open a new connection: ", err)
			}
			defer conn.Close()
		}

		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed to because CPU didn't idle in time: ", err)
		}

		if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		for _, rotation := range []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotate0} {
			if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, rotation); err != nil {
				s.Fatal("Failed to rotate display: ", err)
			}
		}

		histName := "Ash.Rotation.AnimationSmoothness"
		histogram, err := metrics.GetHistogram(ctx, cr, histName)
		if err != nil {
			s.Fatalf("Failed to get histogram %v: %v", histName, err)
		}
		histToReport := histogram
		if prevHist, ok := prevHists[histName]; ok {
			if histToReport, err = histogram.Diff(prevHist); err != nil {
				s.Fatalf("Failed to compute the histogram diff of %v: %v", histName, err)
			}
		}
		prevHists[histName] = histogram
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.%dwindows", histName, currentWindows),
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, histToReport.Mean())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
