// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
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
		Func:         TabletTransitionPerf,
		Desc:         "Measures the animation smoothess of animating to and from tablet mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func waitTopWindowFinishAnimating(ctx context.Context, c *chrome.Conn, windowID int) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		windows, err := ash.GetAllWindows(ctx, c)
		if err != nil {
			return errors.Wrap(err, "failed to get ash windows")
		}

		var windowWithID *ash.Window
		for _, window := range windows {
			if window.ID == windowID {
				windowWithID = window
			}
		}

		if windowWithID == nil {
			return errors.New("did not find the window with the given ID")
		}

		if windowWithID.IsAnimating {
			return errors.New("the top window is still animating")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second})
}

func TabletTransitionPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	const numWindows = 8
	conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, numWindows)
	if err != nil {
		s.Fatal("Failed to create windows: ", err)
	}
	defer conns.Close()

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

	// The top window (first window in the list returned by |ash.GetAllWindow|) needs to be normal window state otherwise no animation will occur.
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

	histograms, err := metrics.Run(ctx, cr, func() error {
		const numRuns = 10
		for i := 0; i < numRuns; i++ {
			if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
				return errors.Wrap(err, "failed to enable tablet mode")
			}

			// Wait for the top window to finish animating before changing states.
			if err := waitTopWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
				return errors.Wrap(err, "failed to wait for top window animation")
			}

			if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
				return errors.Wrap(err, "failed to disable tablet mode")
			}

			if err := waitTopWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
				return errors.Wrap(err, "failed to wait for top window animation")
			}
		}
		return nil
	},
		"Ash.TabletMode.AnimationSmoothness.Enter",
		"Ash.TabletMode.AnimationSmoothness.Exit")
	if err != nil {
		s.Fatal("Failed to run transition or get histogram: ", err)
	}

	pv := perf.NewValues()
	for _, h := range histograms {
		pv.Set(perf.Metric{
			Name:      h.Name,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, h.Mean())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
