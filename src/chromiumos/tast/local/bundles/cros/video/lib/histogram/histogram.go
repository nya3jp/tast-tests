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

// GetInitHistogram returns histogram of MediaGVDInitStatus.
func GetInitHistogram(ctx context.Context, cr *chrome.Chrome) (*metrics.Histogram, error) {
	return metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
}

// WasHWAccelUsed returns whether a video in cr played with HW acceleration.
func WasHWAccelUsed(ctx context.Context, cr *chrome.Chrome, initHistogram *metrics.Histogram) (bool, error) {
	// There are three valid cases.
	// 1. No histogram is updated. This is the case HW Acceleration is disabled due to Chrome flag, --disable-accelerated-video-decode.
	// 2. Histogram is updated with 15. This is the case Chrome tries to initailize VDA but it fails because the codec is not supported on DUT.
	// 3. Histogram is updated with 0. This is the case Chrome sucessfully initializes VDA.

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

// CheckHWAccelUsed checks that video decode accelerator was used while playing a video.
// initHistogram must be obtained by GetInitHistogram before playing the video.
//
// This function is intended to be used like the following:
// initHistogram, err := histogram.GetInitHistogram(ctx, cr)
// if err != nil {
//  	s.Fatal("Failed to get initial histogram: ", err)
// }
// defer histogram.CheckHWAccelUsed(ctx, s, cr, initHistogram)
// ... code for playing a video ...
func CheckHWAccelUsed(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	initHistogram *metrics.Histogram) {
	wasUsed, err := WasHWAccelUsed(ctx, cr, initHistogram)
	if err != nil {
		s.Fatal("Failed to check if hardware acceleration was used", err)
	} else if !wasUsed {
		s.Fatal("Hardware acceleration was not used for playing a video")
	}
}

// CheckHWAccelError checks that no GPU video decoder error occurred while playing video.
func CheckHWAccelError(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	if histogram, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDError); err == nil {
		s.Fatal("GPU video decode error histogram is not empty: ", histogram)
	}
}
