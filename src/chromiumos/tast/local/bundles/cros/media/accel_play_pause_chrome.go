// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/arc/mediasession"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccelPlayPauseChrome,
		Desc:         "Checks the play/pause accelerator will play/pause Chrome",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Timeout:      4 * time.Minute,
		Data: []string{
			"media_session_60sec_test.ogg",
			"media_session_test.html",
		},
	})
}

func AccelPlayPauseChrome(ctx context.Context, s *testing.State) {
	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	args := []string{"--enable-features=HardwareMediaKeyHandling,MediaSessionService,AudioFocusEnforcement"}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(args))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	s.Log("Launching media playback in Chrome")
	conn, err := mediasession.LoadTestPageAndStartPlaying(ctx, cr, server)
	if err != nil {
		s.Fatal("failed to start playback: ", err)
	}
	defer conn.Close()

	s.Log("Creating keyboard event writer")
	k, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("failed to create keyboard event writer: ", err)
	}
	defer k.Close()

	s.Log("Sending play/pause key")
	if err = k.Accel(ctx, "playpause"); err != nil {
		s.Fatal("failed to send play pause key: ", err)
	}

	s.Log("Checking that Chrome is paused")
	must(conn.Exec(ctx, mediasession.CheckChromeIsPaused))

	s.Log("Sending play/pause key")
	if err = k.Accel(ctx, "playpause"); err != nil {
		s.Fatal("failed to send play pause key: ", err)
	}

	s.Log("Checking that Chrome is playing")
	must(conn.Exec(ctx, mediasession.CheckChromeIsPlaying))
}
