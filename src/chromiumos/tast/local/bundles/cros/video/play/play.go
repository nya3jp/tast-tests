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
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/constants"
	"chromiumos/tast/local/media/logging"
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
	ctx, st := timing.Start(ctx, "poll_playback_current_time")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "load_video")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "load_page")
	defer st.End()

	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %v", url)
	}
	return conn, err
}

// playVideo invokes loadVideo(), plays a normal video in video.html, and checks if it has progress.
// videoFile is the file name which is played there.
// baseURL is the base URL which serves video playback testing webpage.
func playVideo(ctx context.Context, cr *chrome.Chrome, videoFile, baseURL string) (string, error) {
	ctx, st := timing.Start(ctx, "play_video")
	defer st.End()

	theURL := baseURL + "/video.html"
	conn, err := loadPage(ctx, cr, theURL)
	if err != nil {
		return theURL, err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := loadVideo(ctx, conn, videoFile); err != nil {
		return theURL, err
	}

	if err := conn.Exec(ctx, "play()"); err != nil {
		return theURL, errors.Wrap(err, "failed to play a video")
	}

	if err := pollPlaybackCurrentTime(ctx, conn, 0.9, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return theURL, errors.Wrap(err, "timed out waiting for playback")
	}

	if err := conn.Exec(ctx, "pause()"); err != nil {
		return theURL, errors.Wrap(err, "failed to pause")
	}

	return theURL, nil
}

// initShakaPlayer initializes Shaka player with video file.
func initShakaPlayer(ctx context.Context, conn *chrome.Conn, mpdFile string) error {
	ctx, st := timing.Start(ctx, "init_shaka_player")
	defer st.End()

	if err := conn.Exec(ctx, fmt.Sprintf("initPlayer(%q)", mpdFile)); err != nil {
		return errors.Wrap(err, "failed to initialize shaka player")
	}
	return nil
}

// waitForShakaPlayerTestDone waits for Shaka player's isTestDone() JS method returns true.
func waitForShakaPlayerTestDone(ctx context.Context, conn *chrome.Conn) error {
	ctx, st := timing.Start(ctx, "wait_for_shaka_player_test_done")
	defer st.End()

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
func playMSEVideo(ctx context.Context, cr *chrome.Chrome, mpdFile, baseURL string) (string, error) {
	ctx, st := timing.Start(ctx, "play_mse_video")
	defer st.End()

	theURL := baseURL + "/shaka.html"
	conn, err := loadPage(ctx, cr, theURL)
	if err != nil {
		return theURL, err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := initShakaPlayer(ctx, conn, mpdFile); err != nil {
		return theURL, err
	}

	if err := waitForShakaPlayerTestDone(ctx, conn); err != nil {
		return theURL, err
	}

	return theURL, nil
}

// seekVideoRepeatedly seeks video numSeeks * numFastSeeks times.
func seekVideoRepeatedly(ctx context.Context, conn *chrome.Conn, numSeeks, numFastSeeks int) error {
	ctx, st := timing.Start(ctx, "seek_video_repeatly")
	defer st.End()

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
	ctx, st := timing.Start(ctx, "play_seek_video")
	defer st.End()

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
	defer conn.CloseTarget(ctx)

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

// snapshotErrorHistogram snapshots the histogram of MediaGVDError.
func snapshotErrorHistogram(ctx context.Context, cr *chrome.Chrome) (errorHistogram *metrics.Histogram, err error) {
	ctx, st := timing.Start(ctx, "snapshot_histogram")
	defer st.End()
	errorHistogram, err = metrics.GetHistogram(ctx, cr, constants.MediaGVDError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get MediaGVDError")
	}
	return
}

// expectErrorHistogram expects video decoder accelerator is used with no error
// code.
func expectErrorHistogram(ctx context.Context, cr *chrome.Chrome, errorHistogram *metrics.Histogram) error {
	if histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaGVDError, errorHistogram, time.Second); err == nil {
		return errors.Errorf("GPU video decode error occurred while playing a video: %v", histogramDiff)
	}
	return nil
}

// checkIfURLUsesPlatformVideoDecoder digs into chrome://media-internals to find
// out if the given theURL is/was played with a platform video decoder (i.e. a
// hardware accelerated video decoder).
// chromeMediaInternalsConn must be a connection to a media-internals tab.
func checkIfURLUsesPlatformVideoDecoder(ctx context.Context, s *testing.State, chromeMediaInternalsConn *chrome.Conn, theURL string) (usesPlatformVideoDecoder bool, err error) {

	checkForPlatformVideoDecoder := fmt.Sprintf(`new Promise((resolve, reject) => {
		const listOfPlayers = document.getElementById('player-list');
		if (listOfPlayers === null || listOfPlayers.length == 0) {
			return reject(new Error('Could not find "player-list" elements'));
		}

		const listOfItems = listOfPlayers.getElementsByTagName('li');
		if (listOfItems === null || listOfItems.length == 0) {
			return reject(new Error('Could not find "li" inside "player-list" element.'));
		}

		let urlFound = false;
		for (i = 0; i < listOfItems.length; i++) {
			const playerFrame = listOfItems[i].getElementsByClassName('player-frame');
			if (playerFrame === undefined || playerFrame.length == 0) {
				continue;
			}

			if (playerFrame[0].innerText == "%s") {
				urlFound = true;
				// Simulate a click to enable the logging for this element.
				playerFrame[0].click();
				break;
			}
		}
		if (!urlFound) {
			console.error('%s url not found');
			return reject(new Error('%s url was not found in chrome:media-internals.'));
		}

		const logTable = document.getElementById('log');
		if (logTable === null) {
			return reject(new Error('Could not find the "log" table.'));
		}

		const logTableRow = logTable.getElementsByTagName('tr');
		if (logTableRow === null || logTableRow.length == 0) {
			return reject(new Error('Could not find log rows.'));
		}

		for (i = 0; i < logTableRow.length; i++) {
			if (logTableRow[i].cells[1].innerHTML == 'is_platform_video_decoder') {
				return resolve(logTableRow[i].cells[2].innerHTML == "true");
			}
		}
		reject(new Error('Did not find is_platform_video_decoder.'));
	})`,
		theURL, theURL, theURL)

	if err := chromeMediaInternalsConn.EvalPromise(ctx, checkForPlatformVideoDecoder, &usesPlatformVideoDecoder); err != nil {
		s.Fatal("Checking chrome://media-internals failed: ", err)
	}
	if err != nil {
		return false, err
	}
	return usesPlatformVideoDecoder, err
}

// TestPlay checks that the video file named filename can be played back using
// a video decode accelerator.
// videotype represents a type of a given video. If it is MSEVideo, filename is a name
// of MPD file.
func TestPlay(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	filename string, videotype VideoType) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer audio.Unmute(ctx)

	chromeMediaInternalsConn, err := cr.NewConn(ctx, "chrome://media-internals")
	if err != nil {
		s.Fatal("Failed to open chrome://media-internals: ", err)
	}
	defer chromeMediaInternalsConn.Close()
	defer chromeMediaInternalsConn.CloseTarget(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	var errorHistogram *metrics.Histogram
	errorHistogram, err = snapshotErrorHistogram(ctx, cr)
	if err != nil {
		s.Fatal("Failed to snapshot error histogram: ", err)
	}

	var playErr error
	var theURL string
	switch videotype {
	case NormalVideo:
		theURL, playErr = playVideo(ctx, cr, filename, server.URL)
	case MSEVideo:
		theURL, playErr = playMSEVideo(ctx, cr, filename, server.URL)
	}
	if playErr != nil {
		s.Fatalf("Failed to play %v (%v): %v", filename, theURL, playErr)
	}

	usesPlatformVideoDecoder, err := checkIfURLUsesPlatformVideoDecoder(ctx, s, chromeMediaInternalsConn, theURL)
	if err != nil {
		s.Fatal("Failed to parse chrome:media-internals: ", err)
	}
	s.Log("usesPlatformVideoDecoder? ", usesPlatformVideoDecoder)

	if !usesPlatformVideoDecoder {
		s.Fatal("Video Decode Accelerator was not used when it was expected to")
	}

	if err := expectErrorHistogram(ctx, cr, errorHistogram); err != nil {
		s.Fatal("Error during histogram check: ", err)
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
