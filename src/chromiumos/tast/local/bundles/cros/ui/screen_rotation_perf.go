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
		Timeout:      3 * time.Minute,
	})
}

func ScreenRotationPerf(ctx context.Context, s *testing.State) {
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

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	pv := perf.NewValues()
	currentWindows := 0
	// Run the screen rotation in overview mode with 2 or 8 windows.
	for _, windows := range []int{2, 8} {
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		defer conns.Close()
		currentWindows = windows

		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed to because CPU didn't idle in time: ", err)
		}

		if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		histograms, err := metrics.Run(ctx, cr, func() error {
			for _, rotation := range []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotate0} {
				if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, rotation); err != nil {
					return errors.Wrap(err, "failed to rotate display")
				}
			}
			return nil
		}, "Ash.Rotation.AnimationSmoothness")
		if err != nil {
			s.Fatal("Failed to rotate display or get histogram: ", err)
		}

		smoothness, err := histograms[0].Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", histograms[0].Name, err)
		}

		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.%dwindows", histograms[0].Name, currentWindows),
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, smoothness)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
