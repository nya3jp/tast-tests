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
	"chromiumos/tast/local/bundles/cros/video/lib/audio"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/histogram"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// VideoType represents a type of video played in TestPlay.
type VideoType int

const (
	// NormalVideo represents a normal video. (i.e. non-MSE video.)
	NormalVideo VideoType = iota
	// MSEVideo represents a video requiring Media Source Extensions (MSE).
	MSEVideo
)

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
		"third_party/shaka-player/shaka-player.compiled.debug.js",
		"third_party/shaka-player/shaka-player.compiled.debug.map",
	}
}

// connWaitForExpr is a wrapper of conn.WaitForExpr to log JavaScript expression to wait for.
func connWaitForExpr(ctx context.Context, conn *chrome.Conn, expr string) error {
	testing.ContextLog(ctx, fmt.Sprintf("Wait for JS expression:%s", expr))
	return conn.WaitForExpr(ctx, expr)
}

// connExec is a wrapper of conn.Exec to log JavaScript expression to execute.
func connExec(ctx context.Context, conn *chrome.Conn, expr string) error {
	testing.ContextLog(ctx, fmt.Sprintf("Execute JS: %s", expr))
	return conn.Exec(ctx, expr)
}

// pollCurrentTime polls JavaScript "currentTime() > threshold" with optioanl PollOptions.
// It also logs each currentTime it polls for debugging.
func pollCurrentTime(ctx context.Context, conn *chrome.Conn, threshold float64, opts *testing.PollOptions) error {
	failToPoll := errors.Errorf("unable to poll currentTime() > %f", threshold)
	testing.ContextLog(ctx, fmt.Sprintf("poll currentTime() > %f", threshold))
	return testing.Poll(ctx, func(ctx context.Context) error {
		t := 0.0
		if err := conn.Eval(ctx, "currentTime()", &t); err != nil {
			return err
		}
		testing.ContextLog(ctx, fmt.Sprintf("currentTime: %f", t))
		if t <= threshold {
			return failToPoll
		}
		return nil
	}, opts)
}

// prepareVideo makes the video specified in videoFile ready to be played, by
// waiting for the document to be ready, loading the video source, and waiting
// until it is ready to play. "play()" can then be called in order to start
// video playback.
func prepareVideo(ctx context.Context, conn *chrome.Conn, videoFile string) error {
	if err := connWaitForExpr(ctx, conn, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "timed out waiting for page load")
	}

	if err := connExec(ctx, conn, fmt.Sprintf("loadVideoSource(%q)", videoFile)); err != nil {
		return errors.Wrap(err, "failed to load a video source")
	}

	if err := connWaitForExpr(ctx, conn, "canplay()"); err != nil {
		return errors.Wrap(err, "timed out waiting for video load")
	}

	return nil
}

// playVideo invokes prepareVideo() then plays a normal video in video.html.
// videoFile is the file name which is played there.
func playVideo(ctx context.Context, conn *chrome.Conn, videoFile string) error {
	if err := prepareVideo(ctx, conn, videoFile); err != nil {
		return err
	}

	if err := connExec(ctx, conn, "play()"); err != nil {
		return errors.Wrap(err, "failed to play a video")
	}

	if err := pollCurrentTime(ctx, conn, 0.9,
		&testing.PollOptions{Timeout: 10 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "timed out waiting for playback")
	}

	if err := connExec(ctx, conn, "pause()"); err != nil {
		return errors.Wrap(err, "failed to pause")
	}

	return nil
}

// playMSEVideo plays an MSE video stream in shaka.html by using shaka player.
// mpdFile is the name of MPD file for the video stream.
func playMSEVideo(ctx context.Context, conn *chrome.Conn, mpdFile string) error {
	if err := connWaitForExpr(ctx, conn, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "timed out waiting for page loaded")
	}

	if err := connExec(ctx, conn, fmt.Sprintf("initPlayer(%q)", mpdFile)); err != nil {
		return errors.Wrap(err, "failed to initialize shaka player")

	}

	rctx, rcancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer rcancel()
	if err := connWaitForExpr(rctx, conn, "isTestDone"); err != nil {
		var messages []interface{}
		if err := conn.Eval(ctx, "errors", &messages); err != nil {
			return errors.Wrap(err, "timed out and failed to get error log")
		}
		return errors.Wrapf(err, "timed out waiting for test completed: %v", messages)
	}

	return nil
}

// seekVideoRandomly invokes prepareVideo() then plays the video referenced by videoFile
// while repeatedly and randomly seeking into it. It returns an error if
// seeking did not succeed for some reason.
func seekVideoRandomly(ctx context.Context, conn *chrome.Conn, videoFile string) error {
	const (
		numSeeks     = 100
		numFastSeeks = 16
	)

	if err := prepareVideo(ctx, conn, videoFile); err != nil {
		return err
	}

	if err := connExec(ctx, conn, "play()"); err != nil {
		return errors.Wrap(err, "failed to play a video")
	}

	for i := 0; i < numSeeks; i++ {
		if err := connExec(ctx, conn, fmt.Sprintf("doFastSeeks(%d)", numFastSeeks)); err != nil {
			return errors.Wrap(err, "failed to fast-seek")
		}

		if err := connWaitForExpr(ctx, conn, "finishedSeeking()"); err != nil {
			return errors.Wrap(err, "timeout while waiting for seek to complete")
		}
	}

	if err := connExec(ctx, conn, "pause()"); err != nil {
		return errors.Wrap(err, "failed to pause")
	}

	return nil
}

// TestPlay checks that the video file named filename can be played back.
// videotype represents a type of a given video. If it is MSEVideo, filename is a name
// of MPD file.
// If mode is CheckHistogram, this function also checks if hardware accelerator
// was used properly.
func TestPlay(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	filename string, videotype VideoType, mode HistogramMode) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer audio.Unmute(ctx)

	// initHistogram and errorHistogram are used in CheckHistogram mode
	var initHistogram, errorHistogram *metrics.Histogram

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

	// Establish a connection to a video play page
	var htmlName string
	switch videotype {
	case NormalVideo:
		htmlName = "video.html"
	case MSEVideo:
		htmlName = "shaka.html"
	}
	conn, err := cr.NewConn(ctx, server.URL+"/"+htmlName)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", htmlName, err)
	}
	defer conn.Close()

	// Play a video
	var playErr error
	switch videotype {
	case NormalVideo:
		playErr = playVideo(ctx, conn, filename)
	case MSEVideo:
		playErr = playMSEVideo(ctx, conn, filename)
	}
	if playErr != nil {
		s.Fatalf("Failed to play %v: %v", filename, playErr)
	}

	if mode == CheckHistogram {
		// Check for MediaGVDInitStatus
		wasUsed, err := histogram.WasHWAccelUsed(ctx, cr, initHistogram, constants.MediaGVDInitStatus, int64(constants.MediaGVDInitSuccess))
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

// TestSeek checks that the video file named filename can be seeked around.
// It will play the video and seek randomly into it 100 times.
func TestSeek(ctx context.Context, s *testing.State, cr *chrome.Chrome, filename string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Establish a connection to a video play page
	const htmlName = "video.html"
	conn, err := cr.NewConn(ctx, server.URL+"/"+htmlName)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", htmlName, err)
	}
	defer conn.Close()

	// Play and seek the video
	if err := seekVideoRandomly(ctx, conn, filename); err != nil {
		s.Fatalf("Failed to play %v: %v", filename, err)
	}
}
