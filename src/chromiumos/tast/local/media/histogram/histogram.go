// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package histogram is a package for common code related to video specific histogram.
package histogram

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// WasHWAccelUsed returns whether HW acceleration is used for certain action.
// initHistogram is the histogram obtained before the action.
// successValue is the bucket value of HW acceleration success case.
// Note that it returns true even if more than one successful count is observed.
// It is because in some cases, we occassionally observed more than one
// successful count (crbug.com/985068). To prevent it from reporting false
// negative result, we relax the condition from successful count == 1 to >= 1,
// regardless it may introuce some false positive result.
// TODO(crbug.com/985068#c5): follow up the improvement plan.
func WasHWAccelUsed(ctx context.Context, tconn *chrome.TestConn, initHistogram *metrics.Histogram, histogramName string, successValue int64) (bool, error) {
	// There are three valid cases.
	// 1. No histogram is updated. This is the case if HW Acceleration is disabled due to Chrome flag, ex. --disable-accelerated-video-decode.
	// 2. Histogram is updated with 15. This is the case if Chrome tries to initailize HW Acceleration but it fails because the codec is not supported on DUT.
	// 3. Histogram is updated with 0. This is the case if Chrome sucessfully initializes HW Acceleration.

	// err is not nil here if HW Acceleration is disabled and then Chrome doesn't try HW Acceleration initialization at all.
	// For the case 1, we pass a short time context to WaitForHistogramUpdate to avoid the whole test context (ctx) from reaching deadline.
	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, tconn, histogramName, initHistogram, 15*time.Second)
	if err != nil {
		// This is the first case; no histogram is updated.
		return false, nil
	}

	testing.ContextLogf(ctx, "Got update to %s histogram: %s", histogramName, histogramDiff)
	if len(histogramDiff.Buckets) > 1 {
		return false, errors.Wrapf(err, "unexpected histogram update: %v", histogramDiff)
	}

	diff := histogramDiff.Buckets[0]
	hwAccelUsed := diff.Min == successValue && diff.Max == successValue+1 && diff.Count >= 1
	if !hwAccelUsed {
		testing.ContextLogf(ctx, "Histogram update: %s, if HW accel were used, it should be [[%d, %d) 1]", histogramDiff, successValue, successValue+1)
	} else if diff.Count > 1 {
		testing.ContextLog(ctx, "Note that more than one successful count is observed: ", diff.Count)
	}

	return hwAccelUsed, nil
}
