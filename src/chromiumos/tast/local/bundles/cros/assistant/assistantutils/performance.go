// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package assistantutils provides utility functions for running assistant tast tests.
package assistantutils

import (
	"fmt"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/metrics"
)

// ProcessHistogram saves histogram data to perf.Values.
func ProcessHistogram(
	histograms []*metrics.Histogram,
	pv *perf.Values, nWindows int,
) error {
	for _, h := range histograms {
		mean, err := h.Mean()
		if err != nil {
			return errors.Wrapf(err, "failed to get mean for histogram %s", h.Name)
		}
		pv.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s.%dwindows", h.Name, nWindows),
				Unit:      "percent",
				Direction: perf.BiggerIsBetter,
			},
			mean,
		)
	}
	return nil
}
