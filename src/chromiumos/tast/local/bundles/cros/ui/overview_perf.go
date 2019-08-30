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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewPerf,
		Desc:         "Measures animation smoothness of entering/exiting the overview mode",
		Contacts:     []string{"mukai@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
	})
}

func meanHistogram(hist *metrics.Histogram) float64 {
	if hist.TotalCount() == 0 {
		return 0
	}
	var sum int64
	for _, bucket := range hist.Buckets {
		sum += (bucket.Max + bucket.Min) * bucket.Count
	}
	return float64(sum) / (float64(hist.TotalCount()) * 2)
}

func addMeanAnimationSmoothness(ctx context.Context, pv *perf.Values, cr *chrome.Chrome, histname string) error {
	histogram, err := metrics.GetHistogram(ctx, cr, histname)
	if err != nil {
		return errors.Wrapf(err, "can't get histogram %s", histname)
	}

	pv.Set(perf.Metric{
		Name:      histname,
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, (meanHistogram(histogram)))
	return nil
}

func OverviewPerf(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Wait for 5 seconds to be stabilized.
	select {
	case <-time.After(5 * time.Second):
		break
	case <-ctx.Done():
		s.Fatal("Failed to wait")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	if tabletMode, err := ash.TabletModeEnabled(ctx, tconn); err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	} else if tabletMode {
		s.Log("Currently tablet devices are not supported. Exiting")
		return
	}

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to open a new connection: ", err)
	}
	defer conn.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the keyboard: ", err)
	}
	defer kb.Close()

	layout, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	for i := 0; i < 10; i++ {
		if err = kb.Accel(ctx, layout.SelectTask); err != nil {
			s.Fatal("Failed to type the overview key: ", err)
		}
		if err = ash.WaitForOverviewModeState(ctx, tconn, true); err != nil {
			s.Fatal("It does not appear to be in the overview mode: ", err)
		}
		if err = kb.Accel(ctx, layout.SelectTask); err != nil {
			s.Fatal("Failed to type the overview key: ", err)
		}
		if err = ash.WaitForOverviewModeState(ctx, tconn, false); err != nil {
			s.Fatal("The overview mode does not end: ", err)
		}
	}

	pv := perf.NewValues()
	for _, histname := range []string{
		"Ash.Overview.AnimationSmoothness.Enter.SingleClamshellMode",
		"Ash.Overview.AnimationSmoothness.Exit.SingleClamshellMode",
	} {
		if err = addMeanAnimationSmoothness(ctx, pv, cr, histname); err != nil {
			s.Fatal("Failed to record perf data: ", err)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
