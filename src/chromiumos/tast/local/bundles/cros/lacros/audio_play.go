// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioPlay,
		Desc:         "Tests basic audio playback on lacros",
		Contacts:     []string{"yuhsuan@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Pre:          launcher.StartedByData(),
		Timeout:      7 * time.Minute, // A lenient limit for launching Lacros Chrome.
		Data:         []string{launcher.DataArtifact, "media_session_60sec_test.ogg", "audio_playback_test.html"},
	})
}

func AudioPlay(ctx context.Context, s *testing.State) {
	l, err := launcher.LaunchLacrosChrome(ctx, s.PreValue().(launcher.PreData))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := l.NewConn(ctx, server.URL+"/audio_playback_test.html")
	if err != nil {
		s.Fatal(err, "failed to open new tab")
	}
	defer conn.Close()

	if err := conn.Eval(ctx, "audio.play()", nil); err != nil {
		s.Fatal("Failed to start playing: ", err)
	}

	if err := conn.WaitForExpr(ctx, "audio.currentTime > 0"); err != nil {
		s.Fatal("Failed to wait for audio to play: ", err)
	}

	if _, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream); err != nil {
		s.Error("Failed to detect running output device: ", err)
	}
}
