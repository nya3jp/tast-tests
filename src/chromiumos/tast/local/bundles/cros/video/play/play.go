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
	// initHistogram and errorHistogram are used in CheckHistogram mode
	var initHistogram, errorHistogram *metrics.Histogram

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	if mode == CheckHistogram {
		var err error
		initHistogram, err = metrics.GetHistogram(ctx, cr, constants.MediaGVDInitStatus)
		if err != nil {
			s.Fatal("Failed to get  MediaGVDInitStatus histogram: ", err)
		}

		errorHistogram, err = metrics.GetHistogram(ctx, cr, constants.MediaGVDError)
		if err != nil {
			s.Fatal("Failed to get MediaGVDError histogram: ", err)
		}
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

	if mode == CheckHistogram {
		// Check for MediaGVDInitStatus
		wasUsed, err := histogram.WasHWAccelUsed(ctx, cr, initHistogram)
		if err != nil {
			s.Fatal("Failed to check for hardware acceleration: ", err)
		} else if !wasUsed {
			s.Fatal("Hardware acceleration was not used for playing a video.")
		}

		// Check for MediaGVDError
		if histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaGVDError,
			errorHistogram, 5*time.Second); err == nil {
			s.Fatal("GPU video decode error occurred while playing a video: ", histogramDiff)
		}
	}
}
