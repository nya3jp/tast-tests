// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemTrayPerf,
		Desc:         "Measures animation smoothness of system tray animations",
		Contacts:     []string{"amehfooz@chromium.org", "tengs@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          ash.LoggedInWith100DummyApps(),
		Timeout:      3 * time.Minute,
	})
}

func SystemTrayPerf(ctx context.Context, s *testing.State) {
	const numRuns = 10
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Click function to toggle collapsed state by clicking on collapse button.
	clickFunc := func(ctx context.Context, obj *ui.Node) error {
		return obj.LeftClick(ctx)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	ash.ShowSystemTray(ctx, tconn)

	// Find the collapse button view bounds.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the root: ", err)
	}

	systemTray, err := root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "UnifiedSystemTrayView"}, time.Second)
	collapseButton, err := systemTray.Descendant(ctx, ui.FindParams{ClassName: "CollapseButton"})
	if err != nil {
		s.Fatal("Failed to find the collapse button: ", err)
	}

	suffixes := [2]string{"TransitionToCollapsed", "TransitionToExpanded"}
	pv := perf.NewValues()

	// Toggle the collapsed state of the system tray for numRuns and record
	// the relevant metrics.
	hists, err := metrics.Run(ctx, tconn, func() error {
		for i := 0; i < numRuns; i++ {
			if err := clickFunc(ctx, collapseButton); err != nil {
				return errors.Wrapf(err, "failed to click collapse button (at step %d)", i)
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
		}
		return nil
	}, "ChromeOS.SystemTray.AnimationSmoothness."+suffixes[0],
		"ChromeOS.SystemTray.AnimationSmoothness."+suffixes[1])
	if err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}
	for _, hist := range hists {
		mean, err := hist.Mean()
		if err != nil {
			s.Fatal("Failed to find the histogram data: ", err)
		}
		pv.Set(perf.Metric{
			Name:      hist.Name,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
