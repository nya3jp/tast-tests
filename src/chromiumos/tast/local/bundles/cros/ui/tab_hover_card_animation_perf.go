// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabHoverCardAnimationPerf,
		Desc:         "Measures the animation smoothness of tab hover card animation",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func TabHoverCardAnimationPerf(ctx context.Context, s *testing.State) {
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

	conn, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to open the tab: ", err)
	}
	if err := conn.Close(); err != nil {
		s.Fatal("Failed to close the connection to the tab: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	bounds := ws[0].BoundsInRoot
	// Use a heuristic offset (30, 30) from the window origin for the first tab.
	tab := coords.NewPoint(bounds.Left + 30, bounds.Top + 30)
	center := bounds.CenterPoint()

	// Stabilize CPU usage.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Error("Failed to wait for system UI to be stabilized: ", err)
	}

	hists, err := metrics.Run(ctx, tconn, func() error {
		if err := mouse.Move(ctx, tconn, center, 0); err != nil {
			s.Fatal("Failed to put mouse to the center: ", err)
		}
		if err := mouse.Move(ctx, tconn, tab, 5 * time.Second); err != nil {
			s.Fatal("Failed to move mouse to the first tab: ", err)
		}
		if err := testing.Sleep(ctx, 5 * time.Second); err != nil {
			s.Fatal("Failed to sleep for 5 seconds: ", err)
		}
		if err := mouse.Move(ctx, tconn, center, 5 * time.Second); err != nil {
			s.Fatal("Failed to move mouse back to the center: ", err)
		}
		return nil
	},
		"Chrome.Tabs.AnimationSmoothness.HoverCard.FadeIn",
		"Chrome.Tabs.AnimationSmoothness.HoverCard.FadeOut")
	if err != nil {
		s.Fatal("Failed to move mouse or get the histogram: ", err)
	}

	pv := perf.NewValues()
	for _, h := range hists {
		mean, err := h.Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
		}
		pv.Set(perf.Metric{
			Name:      h.Name,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
