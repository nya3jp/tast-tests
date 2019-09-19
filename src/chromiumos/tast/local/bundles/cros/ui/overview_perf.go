// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewPerf,
		Desc:         "Measures animation smoothness of entering/exiting the overview mode",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
		Params: []testing.Param{
			{
				Name: "clamshell",
				Val:  "clamshell",
			},
			{
				Name:              "tablet",
				Val:               "touch_view",
				ExtraSoftwareDeps: []string{"tablet_mode"},
			},
		},
	})
}

func OverviewPerf(ctx context.Context, s *testing.State) {
	mode := s.Param().(string)
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--force-tablet-mode="+mode))
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

	for i := 0; i < 10; i++ {
		if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}
		if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			s.Fatal("Failed to exit from the overview mode: ", err)
		}
	}

	suffix := "SingleClamshellMode"
	if inTabletMode, err := ash.TabletModeEnabled(ctx, tconn); err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	} else if inTabletMode {
		suffix = "TabletMode"
	}

	pv := perf.NewValues()
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
			Name:      histName,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, histogram.Mean())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
