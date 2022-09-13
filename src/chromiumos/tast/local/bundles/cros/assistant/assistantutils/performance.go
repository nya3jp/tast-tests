// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/testing"
)

// RecordAnimationPerformance performs an action and record/wait specified metrics.
// - histogramsToWait is a list of histograms to wait for this action.
// - action is a function to perform action (perform animation).
// - getPerfMetricName is a function to return a perf metric name recorded in perf.Values.
func RecordAnimationPerformance(
	ctx context.Context,
	s *testing.State,
	tconn *chrome.TestConn,
	pv *perf.Values,
	histogramsToWait []string,
	action func(context.Context) error,
	getPerfMetricName func(*metrics.Histogram) string,
) error {
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		// Do not block performance test even if we failed to wait cpu idle time.
		// Performance test can run even if cpu is not in idle state.
		s.Log("Failed to wait cpu idle before running performance test. Keep running performance test")
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
