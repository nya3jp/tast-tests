// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
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
		Func:         TabletTransitionPerf,
		Desc:         "Measures the animation smoothess of animating to and from tablet mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
	})
}

func TabletTransitionPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	const numWindows = 8
	for i := 0; i < numWindows; i++ {
		conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open a new connection for a new window: ", err)
		}
		defer conn.Close()
	}

	// The top window (first window in the list returned by |ash.GetAllWindow| needs to be normal window state otherwise no animation will occur.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}

	if _, err := ash.SetWindowState(ctx, tconn, windows[0].ID, ash.WMEventNormal); err != nil {
		s.Fatalf("Failed to set the window (%d): %v", windows[0].ID, err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)

	const numRuns = 10
	for i := 0; i < numRuns; i++ {
		if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enable tablet mode: ", err)
		}

		// Wait for the top window to finish animating before changing states.
		// TODO(crbug.com/1007067): Update this to poll until the window animation completes.
		testing.Sleep(ctx, time.Second)

		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to disable tablet mode: ", err)
		}

		testing.Sleep(ctx, time.Second)
	}

	pv := perf.NewValues()
	for _, histName := range []string{
		"Ash.TabletMode.AnimationSmoothness.Enter",
		"Ash.TabletMode.AnimationSmoothness.Exit",
	} {
		histogram, err := metrics.GetHistogram(ctx, cr, histName)
		if err != nil {
			s.Fatalf("Failed to get histogram %s: %v", histName, err)
		}
		pv.Set(perf.Metric{
			Name:      histName,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, histogram.Mean())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
