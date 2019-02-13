// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/mediasession"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayPauseChrome,
		Desc:         "Checks the play/pause accelerator will play/pause Chrome",
		Contacts:     []string{"beccahughes@chromium.org", "media-dev@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data: []string{
			"media_session_60sec_test.ogg",
			"media_session_test.html",
		},
	})
}

func PlayPauseChrome(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--enable-features=HardwareMediaKeyHandling,MediaSessionService,AudioFocusEnforcement"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	s.Log("Launching media playback in Chrome")
	conn, err := mediasession.LoadTestPageAndStartPlaying(ctx, cr, server.URL+"/media_session_test.html")
	if err != nil {
		s.Fatal("Failed to start playback: ", err)
	}
	defer conn.Close()

	// We use a virtual keyboard here to make sure the play/pause events are always sent.
	s.Log("Creating virtual keyboard event writer")
	k, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create virtual keyboard event writer: ", err)
	}
	defer k.Close()

	// Creating a second page is important because the media keys should work even if the media does not have focus.
	conn2, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to create second page")
	}
	defer conn2.Close()

	s.Log("Sending play/pause key")
	if err = k.Accel(ctx, "playpause"); err != nil {
		s.Fatal("Failed to send play/pause key: ", err)
	}

	s.Log("Checking that Chrome is paused")
	if err = conn.WaitForExpr(ctx, mediasession.CheckChromeIsPaused); err != nil {
		s.Fatal("Failed to check Chrome is paused: ", err)
	}

	s.Log("Sending play/pause key")
	if err = k.Accel(ctx, "playpause"); err != nil {
		s.Fatal("Failed to send play/pause key: ", err)
	}

	s.Log("Checking that Chrome is playing")
	if err = conn.WaitForExpr(ctx, mediasession.CheckChromeIsPlaying); err != nil {
		s.Fatal("Failed to check Chrome is playing: ", err)
	}
}
