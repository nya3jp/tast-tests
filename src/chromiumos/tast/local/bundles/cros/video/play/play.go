// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package play provides common code for playing videos on Chrome.
package play

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/histogram"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// checkHistogramUsed checks that video decode accelerator was used while playing a video.
// initHistogram must be a MediaGVDInitStatus histogram obtained before playing the video.
//
// This function is intended to be used like the following:
//
// initHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
// if err != nil {
//  	s.Fatal("Failed to get initial histogram: ", err)
// }
// defer checkDecodeAccelUsed(ctx, s, cr, initHistogram)
// ... code for playing a video ...
func checkDecodeAccelUsed(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	initHistogram *metrics.Histogram) {
	wasUsed, err := histogram.WasHWAccelUsed(ctx, cr, initHistogram)
	if err != nil {
		s.Fatal("Failed to check if hardware acceleration was used", err)
	} else if !wasUsed {
		s.Fatal("Hardware acceleration was not used for playing a video")
	}
}

// checkHistogramError checks that no GPU video decoder error occurred while playing a video.
// errorHistogram must be a MediaGVDError histogram obtained before playing the video.
// If MediaGVDError histogram didn't exist at the time (i.e. when GetHistogram returned an error),
// errorHistogram should be nil.
//
// This function is intended to be used like the following:
//
// errorHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDError)
// defer checkDecodeAccelError(ctx, s, cr, errorHistogram)
// ... code for playing a video ...
func checkDecodeAccelError(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	errorHistogram *metrics.Histogram) {
	var histogram *metrics.Histogram
	var err error

	if errorHistogram == nil {
		// The case that MediaGVDError histogram was empty before playing a video.
		histogram, err = metrics.GetHistogram(ctx, cr, constants.MediaGVDError)
	} else {
		histogram, err = metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaGVDError,
			errorHistogram, 5*time.Second)
	}

	if err == nil {
		s.Fatal("GPU video decode error occurred while playing a video: ", histogram)
	}
}

type videoPlayMode int

const (
	// NoCheck is a mode that plays a video without checking histograms.
	NoCheck videoPlayMode = iota
	// CheckHistogram is a mode that checks MediaGVD histograms after playing a video.
	CheckHistogram
)

// TestPlay checks that the video file named filename can be played back.
// If mode is CheckHistogram, this function also check if hardware accelerator
// was used properly.
func TestPlay(ctx context.Context, s *testing.State, filename string, mode videoPlayMode) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	if mode == CheckHistogram {
		initHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
		if err != nil {
			s.Fatal("Failed to get initial histogram: ", err)
		}
		defer checkDecodeAccelUsed(ctx, s, cr, initHistogram)

		errorHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaGVDError)
		defer checkDecodeAccelError(ctx, s, cr, errorHistogram)
	}

	conn, err := cr.NewConn(ctx, server.URL+"/video.html")
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "script_ready"); err != nil {
		s.Fatal("Timed out waiting for player ready: ", err)
	}

	if err := conn.Exec(ctx, fmt.Sprintf("loadVideoSource(%q)", filename)); err != nil {
		s.Fatal("Failed loadVideoSource: ", err)
	}

	if err := conn.WaitForExpr(ctx, "canplay()"); err != nil {
		s.Fatal("Timed out waiting for video load: ", err)
	}

	if err := conn.Exec(ctx, "play()"); err != nil {
		s.Fatal("Failed play: ", err)
	}

	if err := conn.WaitForExpr(ctx, "currentTime() > 0.9"); err != nil {
		s.Fatal("Timed out waiting for playback: ", err)
	}

	if err := conn.Exec(ctx, "pause()"); err != nil {
		s.Fatal("Failed pause: ", err)
	}
}
