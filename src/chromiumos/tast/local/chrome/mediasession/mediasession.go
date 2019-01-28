// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mediasession contains common utilities to help writing media session tests.
package mediasession

import (
	"context"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
)

const (
	// CheckChromeIsPlaying is a JS expression that can be evaluated in the connection returned by LoadTestPageAndStartPlaying to check if the audio element on the test page is playing.
	CheckChromeIsPlaying = "!audio.paused"

	// CheckChromeIsPaused is a JS expression that can be evaluated in the connection returned by LoadTestPageAndStartPlaying to check if the audio element on the test page is paused.
	CheckChromeIsPaused = "audio.paused"
)

// LoadTestPageAndStartPlaying starts the media session test page in Chrome and checks that it
// has successfully started playing.
func LoadTestPageAndStartPlaying(ctx context.Context, cr *chrome.Chrome, sr *httptest.Server) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, sr.URL+"/media_session_test.html")
	if err != nil {
		return nil, err
	}

	if err := conn.Exec(ctx, "audio.play()"); err != nil {
		conn.Close()
		return nil, err
	}

	if err := conn.WaitForExpr(ctx, "audio.currentTime > 0"); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
