// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package assistantutils provides utility functions for running assistant tast tests.
package assistantutils

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/cpu"
)

// RecordAnimationPerformance performs an action and record/wait specified metrics
func RecordAnimationPerformance(
	ctx context.Context,
	tconn *chrome.TestConn,
	pv *perf.Values,
	histogramsToWait []string,
	action func(context.Context) error,
	getPerfMetricName func(*metrics.Histogram) string,
) error {
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait cpu idle before running performance test")
	}

	histograms, err := metrics.RunAndWaitAll(
		ctx, tconn, 10*time.Second, action, histogramsToWait...,
	)
	if err != nil {
		return errors.Wrap(err, "failed to run animation and wait histograms")
	}

	if err := processHistogramInternal(histograms, pv, getPerfMetricName); err != nil {
		return errors.Wrap(err, "failed to process histogram")
	}

	return nil
}

// ProcessHistogram saves histogram data to perf.Values.
func ProcessHistogram(
	histograms []*metrics.Histogram,
	pv *perf.Values, nWindows int,
) error {
	return processHistogramInternal(histograms, pv, func(h *metrics.Histogram) string {
		return fmt.Sprintf("%s.%dwindows", h.Name, nWindows)
	})
}

func processHistogramInternal(
	histograms []*metrics.Histogram, pv *perf.Values,
	getPerfMetricName func(*metrics.Histogram) string,
) error {
	for _, h := range histograms {
		mean, err := h.Mean()
		if err != nil {
			return errors.Wrapf(err, "failed to get mean for histogram %s", h.Name)
		}
		pv.Set(
			perf.Metric{
				Name:      getPerfMetricName(h),
				Unit:      "percent",
				Direction: perf.BiggerIsBetter,
			},
			mean,
		)
	}
	return nil
}
