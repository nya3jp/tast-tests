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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/histogram"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// VideoType represents a type of videos played in TestPlay.
type VideoType int

const (
	// Normal represents a normal video. (i.e. non-MSE video.)
	Normal VideoType = iota
	// MSEVideo represents a video requiring Media Source Extensions (MSE).
	MSEVideo
)

// getHTMLName returns a name of an HTML file that can play a given type of videos.
func (t VideoType) getHTMLName() string {
	switch t {
	case MSEVideo:
		return "/shaka.html"
	default:
		return "/video.html"
	}
}

// HistogramMode represents a mode of TestPlay.
type HistogramMode int

const (
	// NoCheckHistogram is a mode that plays a video without checking histograms.
	NoCheckHistogram HistogramMode = iota
	// CheckHistogram is a mode that checks MediaGVD histograms after playing a video.
	CheckHistogram
)

// MSEDataFiles returns a list of required files that tests that play MSE videos.
func MSEDataFiles() []string {
	return []string{
		"shaka.html",
		"third_party/shaka-player.compiled.debug.js",
		"third_party/shaka-player.compiled.debug.map",
	}
}

// playVideo plays a normal video in video.html.
// videoname is the file name which is played there.
func playVideo(ctx context.Context, conn *chrome.Conn, videoname string) error {
	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "timed out waiting for page load")
	}

	if err := conn.Exec(ctx, fmt.Sprintf("loadVideoSource(%q)", videoname)); err != nil {
		return errors.Wrap(err, "failed to load a video source")
	}

	if err := conn.WaitForExpr(ctx, "canplay()"); err != nil {
		return errors.Wrap(err, "timed out waiting for video load")
	}

	if err := conn.Exec(ctx, "play()"); err != nil {
		return errors.Wrap(err, "failed to play a video")
	}

	if err := conn.WaitForExpr(ctx, "currentTime() > 0.9"); err != nil {
		return errors.Wrap(err, "timed out waiting for playback")
	}

	if err := conn.Exec(ctx, "pause()"); err != nil {
		return errors.Wrap(err, "failed to pause")
	}

	return nil
}

// playMSEVideo plays an MSE video stream in shaka.html by using shaka player.
// mpdname is the name of MPD file for a video stream.
func playMSEVideo(ctx context.Context, conn *chrome.Conn, mpdname string) error {
	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "timed out waiting for page loaded")
	}

	if err := conn.Exec(ctx, fmt.Sprintf("initPlayer(%q)", mpdname)); err != nil {
		return errors.Wrap(err, "failed to initialize shaka player")

	}

	rctx, rcancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer rcancel()
	if err := conn.WaitForExpr(rctx, "isTestDone"); err != nil {
		var messages []interface{}
		if err := conn.Eval(ctx, "errors", &messages); err != nil {
			return errors.Wrapf(err, "timed out and failed to get error log.")
		}
		return errors.Wrapf(err, "timed out waiting for test completed: %v", messages)
	}

	return nil
}

// TestPlay checks that the video file named filename can be played back.
// videotype represents a type of a given video. If it is MSEVideo, filename is a name
// of MPD file.
// If mode is CheckHistogram, this function also checks if hardware accelerator
// was used properly.
func TestPlay(ctx context.Context, s *testing.State, filename string, videotype VideoType, mode HistogramMode) {
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
			s.Fatal("Failed to get MediaGVDInitStatus histogram: ", err)
		}

		errorHistogram, err = metrics.GetHistogram(ctx, cr, constants.MediaGVDError)
		if err != nil {
			s.Fatal("Failed to get MediaGVDError histogram: ", err)
		}
	}

	conn, err := cr.NewConn(ctx, server.URL+videotype.getHTMLName())
	if err != nil {
		s.Fatal("Failed to open a video play page: ", err)
	}
	defer conn.Close()

	var playErr error
	switch videotype {
	case Normal:
		playErr = playVideo(ctx, conn, filename)
	case MSEVideo:
		playErr = playMSEVideo(ctx, conn, filename)
	}
	if playErr != nil {
		s.Fatal("Failed to play a video: ", playErr)
	}

	if mode == CheckHistogram {
		// Check for MediaGVDInitStatus
		wasUsed, err := histogram.WasHWAccelUsed(ctx, cr, initHistogram)
		if err != nil {
			s.Fatal("Failed to check for hardware acceleration: ", err)
		} else if !wasUsed {
			s.Fatal("Hardware acceleration was not used for playing a video")
		}

		// Check for MediaGVDError
		if histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaGVDError,
			errorHistogram, 5*time.Second); err == nil {
			s.Fatal("GPU video decode error occurred while playing a video: ", histogramDiff)
		}
	}
}
