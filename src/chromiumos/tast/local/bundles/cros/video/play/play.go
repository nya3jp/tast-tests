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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/chrome"
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

// VerifyHWAcceleratorMode represents a mode of TestPlay.
type VerifyHWAcceleratorMode int

const (
	// NoVerifyHWAcceleratorUsed is a mode that plays a video without verifying
	// hardware accelerator usage.
	NoVerifyHWAcceleratorUsed VerifyHWAcceleratorMode = iota
	// VerifyHWAcceleratorUsed is a mode that verifies a video is played using a
	// hardware accelerator.
	VerifyHWAcceleratorUsed
	// VerifyNoHWAcceleratorUsed is a mode that verifies a video is not played
	// using a hardware accelerator, i.e. it's using software decoding.
	VerifyNoHWAcceleratorUsed
)

// MSEDataFiles returns a list of required files that tests that play MSE videos.
func MSEDataFiles() []string {
	return []string{
		"shaka.html",
		"third_party/shaka-player/shaka-player.compiled.debug.js",
		"third_party/shaka-player/shaka-player.compiled.debug.map",
	}
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
// url is the URL of the video playback testing webpage.
func playVideo(ctx context.Context, cr *chrome.Chrome, videoFile, url string) error {
	ctx, st := timing.Start(ctx, "play_video")
	defer st.End()

	conn, err := loadPage(ctx, cr, url)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.EvalPromise(ctx, fmt.Sprintf("play(%q)", videoFile), nil); err != nil {
		return err
	}

	return nil
}

// playMSEVideo plays an MSE video stream via Shaka player, and checks its play progress.
// mpdFile is the name of MPD file for the video stream.
// url is the URL of the shaka player webpage.
func playMSEVideo(ctx context.Context, cr *chrome.Chrome, mpdFile, url string) error {
	ctx, st := timing.Start(ctx, "play_mse_video")
	defer st.End()

	conn, err := loadPage(ctx, cr, url)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.EvalPromise(ctx, fmt.Sprintf("play_shaka(%q)", mpdFile), nil); err != nil {
		return err
	}

	return nil
}

// seekVideoRepeatedly seeks video numSeeks times.
func seekVideoRepeatedly(ctx context.Context, conn *chrome.Conn, numSeeks int) error {
	ctx, st := timing.Start(ctx, "seek_video_repeatly")
	defer st.End()
	prevFinishedSeeks := 0
	for i := 0; i < numSeeks; i++ {
		finishedSeeks := 0
		if err := conn.EvalPromise(ctx, "randomSeek()", &finishedSeeks); err != nil {
			// If the test times out, EvalPromise() might be interrupted and return
			// zero finishedSeeks, in that case used the last known good amount.
			if finishedSeeks == 0 {
				finishedSeeks = prevFinishedSeeks
			}
			return errors.Wrapf(err, "Error while seeking, completed %d/%d seeks", finishedSeeks, numSeeks)
		}
		if finishedSeeks == numSeeks {
			break
		}
		prevFinishedSeeks = finishedSeeks
	}

	return nil
}

// playSeekVideo invokes loadVideo() then plays the video referenced by videoFile
// while repeatedly and randomly seeking into it numSeeks. It returns an error if
// seeking did not succeed for some reason.
// videoFile is the file name which is played and seeked there.
// baseURL is the base URL which serves video playback testing webpage.
func playSeekVideo(ctx context.Context, cr *chrome.Chrome, videoFile, baseURL string, numSeeks int) error {
	ctx, st := timing.Start(ctx, "play_seek_video")
	defer st.End()

	// Establish a connection to a video play page
	conn, err := loadPage(ctx, cr, baseURL+"/video.html")
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.EvalPromise(ctx, fmt.Sprintf("play(%q)", videoFile), nil); err != nil {
		return err
	}
	if err := seekVideoRepeatedly(ctx, conn, numSeeks); err != nil {
		return err
	}
	return nil
}

// TestPlay checks that the video file named filename can be played using Chrome.
// videotype represents a type of a given video. If it is MSEVideo, filename is a name
// of MPD file.
// If mode is VerifyHWAcceleratorUsed, this function also checks if hardware accelerator was used.
func TestPlay(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	filename string, videotype VideoType, mode VerifyHWAcceleratorMode) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return err
	}
	defer vl.Close()

	if err := audio.Mute(ctx); err != nil {
		return err
	}
	defer audio.Unmute(ctx)

	var chromeMediaInternalsConn *chrome.Conn
	if mode != NoVerifyHWAcceleratorUsed {
		chromeMediaInternalsConn, err = decode.OpenChromeMediaInternalsPageAndInjectJS(ctx, cr, s.DataPath("chrome_media_internals_utils.js"))
		if err != nil {
			return errors.Wrap(err, "failed to open chrome://media-internals")
		}
		defer chromeMediaInternalsConn.Close()
		defer chromeMediaInternalsConn.CloseTarget(ctx)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	var playErr error
	var url string
	switch videotype {
	case NormalVideo:
		url = server.URL + "/video.html"
		playErr = playVideo(ctx, cr, filename, url)
	case MSEVideo:
		url = server.URL + "/shaka.html"
		playErr = playMSEVideo(ctx, cr, filename, url)
	}
	if playErr != nil {
		return errors.Wrapf(err, "failed to play %v (%v): %v", filename, url, playErr)
	}

	if mode == NoVerifyHWAcceleratorUsed {
		// Early return ig no verification is needed.
		return nil
	}

	usesPlatformVideoDecoder, err := decode.URLUsesPlatformVideoDecoder(ctx, chromeMediaInternalsConn, server.URL)
	if err != nil {
		return errors.Wrap(err, "failed to parse chrome:media-internals")
	}
	s.Log("usesPlatformVideoDecoder? ", usesPlatformVideoDecoder)

	if mode == VerifyHWAcceleratorUsed && !usesPlatformVideoDecoder {
		return errors.New("video decode acceleration was not used when it was expected to")
	}
	if mode == VerifyNoHWAcceleratorUsed && usesPlatformVideoDecoder {
		return errors.New("software decoding was not used when it was expected to")
	}
	return nil
}

// TestSeek checks that the video file named filename can be seeked around.
// It will play the video and seek randomly into it numSeeks times.
func TestSeek(ctx context.Context, httpHandler http.Handler, cr *chrome.Chrome, filename string, numSeeks int) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return err
	}
	defer vl.Close()

	server := httptest.NewServer(httpHandler)
	defer server.Close()

	if err := playSeekVideo(ctx, cr, filename, server.URL, numSeeks); err != nil {
		return errors.Wrapf(err, "failed to play %v (%v): %v", filename, server.URL, err)
	}
	return nil
}
