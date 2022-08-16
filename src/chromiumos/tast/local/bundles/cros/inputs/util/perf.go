// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/metrics"
)

// PercentSamplesBelow returns the percentage of UMA histogram samples whose buckets that are below a certain threshold.
func PercentSamplesBelow(h *metrics.Histogram, threshold int64) (float64, error) {
	if h.TotalCount() == 0 {
		return 0, errors.New("no histogram data")
	}
	var total = int64(0)
	for _, bucket := range h.Buckets {
		if bucket.Min < threshold {
			total += bucket.Count
		}
	}
	return float64(total) / float64(h.TotalCount()) * 100, nil
}
