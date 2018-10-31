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

	"chromiumos/tast/local/bundles/cros/video/lib/histogram"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// playVideo checks that the video file named filename can be played back.
// If checkHWAccelHistogram is true, this function also check if hardware accelerator
// was used properly.
func playVideo(ctx context.Context, s *testing.State, filename string,
	checkHWAccelHistogram bool) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	if checkHWAccelHistogram {
		initHistogram, err := histogram.GetInitHistogram(ctx, cr)
		if err != nil {
			s.Fatal("Failed to get initial histogram: ", err)
		}
		defer histogram.CheckHWAccelUsed(ctx, s, cr, initHistogram)
		defer histogram.CheckHWAccelError(ctx, s, cr)
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

// TestPlay checks that the video file named filename can be played back.
// This function is used in video.Play* tests.
func TestPlay(ctx context.Context, s *testing.State, filename string) {
	playVideo(ctx, s, filename, false)
}

// ChromeDecodeAccelUsed checks that video decode accelerator is used
// when the video file named filename is played back.
// This function is used in video.ChromeDecodeAccelUsed* tests.
func ChromeDecodeAccelUsed(ctx context.Context, s *testing.State, filename string) {
	playVideo(ctx, s, filename, true)
}

// CrosVideo checks that a video in crosvideo.appspot.com can be played with
// video decode accelerator.
// codec is used as a value of a URL parameter in crosvideo.appspot.com.
// This function is used in video.ChromeDecodeAccelUsed*MSE tests.
func CrosVideo(ctx context.Context, s *testing.State, codec string) {
	crosVideoURL := "http://crosvideo.appspot.com/?cycle=true&mute=true&codec=" + codec

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	initHistogram, err := histogram.GetInitHistogram(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}
	defer histogram.CheckHWAccelUsed(ctx, s, cr, initHistogram)
	defer histogram.CheckHWAccelError(ctx, s, cr)

	conn, err := cr.NewConn(ctx, crosVideoURL)
	if err != nil {
		s.Fatal("Failed to open crosvideo page: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "document.readyState == 'complete'"); err != nil {
		s.Fatal("Timed out waiting for player ready: ", err)
	}

	// Run video player for 30 seconds.
	time.Sleep(30 * time.Second)
}
