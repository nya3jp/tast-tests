// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package histogram is a package for common code related to video specific histogram.
package histogram

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// WasHWAccelUsed returns whether a video in cr played with HW acceleration.
// initHistogram is a MediaGVDInitStatus histogram obtained before playing a video.
func WasHWAccelUsed(ctx context.Context, cr *chrome.Chrome, initHistogram *metrics.Histogram) (bool, error) {
	// There are three valid cases.
	// 1. No histogram is updated. This is the case if HW Acceleration is disabled due to Chrome flag, --disable-accelerated-video-decode.
	// 2. Histogram is updated with 15. This is the case if Chrome tries to initailize VDA but it fails because the codec is not supported on DUT.
	// 3. Histogram is updated with 0. This is the case if Chrome sucessfully initializes VDA.

	// err is not nil here if HW Acceleration is disabled and then Chrome doesn't try VDA initialization at all.
	// For the case 1, we pass a short time context to WaitForHistogramUpdate to avoid the whole test context (ctx) from reaching deadline.
	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaGVDInitStatus, initHistogram, 5*time.Second)
	if err != nil {
		// This is the first case; no histogram is updated.
		return false, nil
	}

	testing.ContextLogf(ctx, "Got update to %s histogram: %v", constants.MediaGVDInitStatus, histogramDiff.Buckets)
	if len(histogramDiff.Buckets) > 1 {
		return false, errors.Wrapf(err, "unexpected histogram update: %v", histogramDiff)
	}

	// If HW acceleration is used, the sole bucket is {0, 1, X}.
	diff := histogramDiff.Buckets[0]
	hwAccelUsed := diff.Min == constants.MediaGVDInitSuccess && diff.Max == constants.MediaGVDInitSuccess+1
	return hwAccelUsed, nil
}
