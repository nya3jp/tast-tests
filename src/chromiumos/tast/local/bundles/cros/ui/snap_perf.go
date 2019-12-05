// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/errors"
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
	defer tconn.Close()

	conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open a new connection: ", err)
	}
	defer conn.Close()

	// Reset the tablet mode state at the end to the original state.
	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)

	// Initialize the tablet mode state to clamshell mode to begin as the device may be tablet only.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable tablet mode: ", err)
	}

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
			return errors.Wrap(err, "failed to wait for top window animation")
		}

		// Restore the normal state bounds, as no animation stats will be logged if the window size does not change.
		if _, err := ash.SetWindowState(ctx, tconn, windows[0].ID, ash.WMEventNormal); err != nil {
			s.Fatalf("Failed to set the window (%d): %v", windows[0].ID, err)
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}

		// Snap the window to the right.
		if _, err := ash.SetWindowState(ctx, tconn, windows[0].ID, ash.WMEventSnapRight); err != nil {
			s.Fatalf("Failed to set the window (%d): %v", windows[0].ID, err)
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}
		return nil
	},
		"Ash.Window.AnimationSmoothness.Snap")
	if err != nil {
		s.Fatal("Failed to snap window or get histogram: ", err)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      histograms[0].Name,
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, histograms[0].Mean())

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
