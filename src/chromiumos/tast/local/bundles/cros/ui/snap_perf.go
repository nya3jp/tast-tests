// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

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
		Func:         SnapPerf,
		Desc:         "Measures the animation smoothess of snapping windows in clamshell mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func SnapPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open a new connection: ", err)
	}
	defer conn.Close()

	tm, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer tm.Close(ctx)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	histograms, err := metrics.Run(ctx, cr, func() error {
		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain the window list: ", err)
		}

		// Snap the window to the left.
		if _, err := ash.SetWindowState(ctx, tconn, windows[0].ID, ash.WMEventSnapLeft); err != nil {
			s.Fatalf("Failed to set the window (%d): %v", windows[0].ID, err)
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
			s.Fatal("Failed to wait for top window animation: ", err)
		}

		// Restore the normal state bounds, as no animation stats will be logged if the window size does not change.
		if _, err := ash.SetWindowState(ctx, tconn, windows[0].ID, ash.WMEventNormal); err != nil {
			s.Fatalf("Failed to set the window (%d): %v", windows[0].ID, err)
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
			s.Fatal("Failed to wait for top window animation: ", err)
		}

		// Snap the window to the right.
		if _, err := ash.SetWindowState(ctx, tconn, windows[0].ID, ash.WMEventSnapRight); err != nil {
			s.Fatalf("Failed to set the window (%d): %v", windows[0].ID, err)
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
			s.Fatal("Failed to wait for top window animation: ", err)
		}
		return nil
	},
		"Ash.Window.AnimationSmoothness.Snap")
	if err != nil {
		s.Fatal("Failed to snap window or get histogram: ", err)
	}

	smoothness, err := histograms[0].Mean()
	if err != nil {
		s.Fatalf("Failed to get mean for histogram %s: %v", histograms[0].Name, err)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      histograms[0].Name,
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, smoothness)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
