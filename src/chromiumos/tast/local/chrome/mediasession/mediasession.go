// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mediasession contains common utilities to help writing media session tests.
package mediasession

import (
	"context"

	"chromiumos/tast/local/chrome"
)

const (
	// CheckChromeIsPlaying is a JS expression that can be evaluated in the connection returned by LoadTestPageAndStartPlaying to check if the audio element on the test page is playing.
	CheckChromeIsPlaying = "!audio.paused"

	// CheckChromeIsPaused is a JS expression that can be evaluated in the connection returned by LoadTestPageAndStartPlaying to check if the audio element on the test page is paused.
	CheckChromeIsPaused = "audio.paused"
)

// LoadTestPageAndStartPlaying opens the media session test page in Chrome and checks that it
// has successfully started playing. The caller is responsible for closing the returned chrome.Conn.
// Tests should symlink data/media_session_test.html into their own data directory and pass the URL
// at which is available via the url argument.
func LoadTestPageAndStartPlaying(ctx context.Context, cr *chrome.Chrome, url string) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, url)
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
