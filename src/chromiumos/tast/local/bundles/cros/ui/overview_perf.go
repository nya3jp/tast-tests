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

	ash.WaitForSystemUIStabilized(ctx)

	for i := 0; i < 10; i++ {
		if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("It does not appear to be in the overview mode: ", err)
		}
		if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			s.Fatal("It does not appear to be in the overview mode: ", err)
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
