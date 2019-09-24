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
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabletTransitionPerf,
		Desc:         "Measures animation smoothness of entering/exiting tablet mode",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"informational", "group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
	})
}

func TabletTransitionPerf(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to open a new connection: ", err)
	}
	defer conn.Close()

	if err = ash.WaitForSystemUIStabilized(ctx); err != nil {
		s.Fatal("Failed to wait for system UI to be stabilized: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	// Use a total of 8 windows for this test.
	for i := 0; i < 7; i++ {
		conn, err := cr.NewConn(ctx, "about:blank", cdputil.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open a new connection for a new window: ", err)
		}
		defer conn.Close()
	}

	if _, err := ash.SetActiveWindowState(ctx, tconn, ash.WMEventNormal); err != nil {
		s.Fatal("Failed to set active window state: ", err)
	}

	for i := 0; i < 10; i++ {
		if err = ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enable tablet mode: ", err)
		}

		testing.Sleep(ctx, time.Second)

		if err = ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
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
