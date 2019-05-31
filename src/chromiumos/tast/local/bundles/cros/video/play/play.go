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
	"chromiumos/tast/timing"
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

// pollPlaybackCurrentTime polls JavaScript "currentTime() > threshold" with optioanl PollOptions.
// If it fails to poll for condition, it emits error with currentTime() value attached.
func pollPlaybackCurrentTime(ctx context.Context, conn *chrome.Conn, threshold float64, opts *testing.PollOptions) error {
	defer timing.Start(ctx, "poll_playback_current_time").End()
	return testing.Poll(ctx, func(ctx context.Context) error {
		var t float64
		if err := conn.Eval(ctx, "currentTime()", &t); err != nil {
			return err
		}
		if t <= threshold {
			return errors.Errorf("currentTime (%f) is below threshold (%f)", t, threshold)
		}
		return nil
	}, opts)
}

// loadVideo makes the video specified in videoFile ready to be played, by
// waiting for the document to be ready, loading the video source, and waiting
// until it is ready to play. "play()" can then be called in order to start
// video playback.
func loadVideo(ctx context.Context, conn *chrome.Conn, videoFile string) error {
	defer timing.Start(ctx, "load_video").End()

	if err := conn.Exec(ctx, fmt.Sprintf("loadVideoSource(%q)", videoFile)); err != nil {
		return errors.Wrap(err, "failed to load a video source")
	}

	if err := conn.WaitForExpr(ctx, "canplay()"); err != nil {
		return errors.Wrap(err, "timed out waiting for video load")
	}

	return nil
}

// loadPage opens a new tab to load the specified webpage.
// Note that if err != nil, conn is nil.
func loadPage(ctx context.Context, cr *chrome.Chrome, url string) (*chrome.Conn, error) {
	defer timing.Start(ctx, "load_page").End()
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %v", url)
	}
	return conn, err
}

// playVideo invokes loadVideo(), plays a normal video in video.html, and checks if it has progress.
// videoFile is the file name which is played there.
// baseURL is the base URL which serves video playback testing webpage.
func playVideo(ctx context.Context, cr *chrome.Chrome, videoFile, baseURL string) error {
	defer timing.Start(ctx, "play_video").End()

	conn, err := loadPage(ctx, cr, baseURL+"/video.html")
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := loadVideo(ctx, conn, videoFile); err != nil {
		return err
	}

	if err := conn.Exec(ctx, "play()"); err != nil {
		return errors.Wrap(err, "failed to play a video")
	}

	if err := pollPlaybackCurrentTime(ctx, conn, 0.9, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "timed out waiting for playback")
	}

	if err := conn.Exec(ctx, "pause()"); err != nil {
		return errors.Wrap(err, "failed to pause")
	}

	return nil
}

// initShakaPlayer initializes Shaka player with video file.
func initShakaPlayer(ctx context.Context, conn *chrome.Conn, mpdFile string) error {
	defer timing.Start(ctx, "init_shaka_player").End()

	if err := conn.Exec(ctx, fmt.Sprintf("initPlayer(%q)", mpdFile)); err != nil {
		return errors.Wrap(err, "failed to initialize shaka player")

	}

	return nil
}

// waitForShakaPlayerTestDone waits for Shaka player's isTestDone() JS method returns true.
func waitForShakaPlayerTestDone(ctx context.Context, conn *chrome.Conn) error {
	defer timing.Start(ctx, "wait_for_shaka_player_test_done").End()

	// rctx is a shorten ctx to reserve 3 second in case to extract errors in JavaScript.
	rctx, rcancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer rcancel()
	if err := conn.WaitForExpr(rctx, "isTestDone"); err != nil {
		var messages []interface{}
		if err := conn.Eval(ctx, "errors", &messages); err != nil {
			return errors.Wrap(err, "timed out getting error log")
		}
		return errors.Wrapf(err, "timed out waiting for test completed: %v", messages)
	}
	return nil
}

// playMSEVideo plays an MSE video stream via Shaka player, and checks its play progress.
// mpdFile is the name of MPD file for the video stream.
// baseURL is the base URL which serves shaka player webpage.
func playMSEVideo(ctx context.Context, cr *chrome.Chrome, mpdFile, baseURL string) error {
	defer timing.Start(ctx, "play_mse_video").End()

	conn, err := loadPage(ctx, cr, baseURL+"/shaka.html")
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := initShakaPlayer(ctx, conn, mpdFile); err != nil {
		return err
	}

	if err := waitForShakaPlayerTestDone(ctx, conn); err != nil {
		return err
	}

	return nil
}

// seekVideoRepeatedly seeks video numSeeks * numFastSeeks times.
func seekVideoRepeatedly(ctx context.Context, conn *chrome.Conn, numSeeks, numFastSeeks int) error {
	defer timing.Start(ctx, "seek_video_repeatly").End()
	for i := 0; i < numSeeks; i++ {
		if err := conn.Exec(ctx, fmt.Sprintf("doFastSeeks(%d)", numFastSeeks)); err != nil {
			return errors.Wrap(err, "failed to fast-seek")
		}

		if err := conn.WaitForExpr(ctx, "finishedSeeking()"); err != nil {
			return errors.Wrap(err, "timeout while waiting for seek to complete")
		}
	}

	return nil
}

// playSeekVideo invokes loadVideo() then plays the video referenced by videoFile
// while repeatedly and randomly seeking into it. It returns an error if
// seeking did not succeed for some reason.
// videoFile is the file name which is played and seeked there.
// baseURL is the base URL which serves video playback testing webpage.
func playSeekVideo(ctx context.Context, cr *chrome.Chrome, videoFile, baseURL string) error {
	defer timing.Start(ctx, "play_seek_video").End()

	const (
		numSeeks     = 100
		numFastSeeks = 16
	)

	// Establish a connection to a video play page
	conn, err := loadPage(ctx, cr, baseURL+"/video.html")
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := loadVideo(ctx, conn, videoFile); err != nil {
		return err
	}

	if err := conn.Exec(ctx, "play()"); err != nil {
		return errors.Wrap(err, "failed to play a video")
	}

	if err := seekVideoRepeatedly(ctx, conn, numSeeks, numFastSeeks); err != nil {
		return err
	}

	if err := conn.Exec(ctx, "pause()"); err != nil {
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

	// Play a video
	var playErr error
	switch videotype {
	case NormalVideo:
		playErr = playVideo(ctx, cr, filename, server.URL)
	case MSEVideo:
		playErr = playMSEVideo(ctx, cr, filename, server.URL)
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

	// Play and seek the video
	if err := playSeekVideo(ctx, cr, filename, server.URL); err != nil {
		s.Fatalf("Failed to play %v: %v", filename, err)
	}
}
