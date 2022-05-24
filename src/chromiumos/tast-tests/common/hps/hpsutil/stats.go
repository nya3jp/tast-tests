// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hpsutil

import (
	"math"

	"chromiumos/tast/errors"
)

// PercentileForSortedData returns the given percentile from c, or an error. Expects data to be sorted.
func PercentileForSortedData(c []float64, percentile int) (float64, error) {
	if len(c) == 0 {
		return math.NaN(), errors.New("empty input")
	}

	if percentile < 0 || percentile >= 100 {
		return math.NaN(), errors.Errorf("invalid percentile bounds: %d", percentile)
	}

	return c[len(c)*percentile/100], nil
}
