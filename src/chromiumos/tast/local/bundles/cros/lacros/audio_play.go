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
	"chromiumos/tast/local/chrome/lacros/faillog"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioPlay,
		Desc:         "Tests basic audio playback on lacros",
		Contacts:     []string{"yuhsuan@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosBypassPermissions",
		Timeout:      7 * time.Minute, // A lenient limit for launching Lacros Chrome.
		Data:         []string{"media_session_60sec_test.ogg", "audio_playback_test.html"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
		}, {
			Name:              "unstable",
			ExtraSoftwareDeps: []string{"lacros_unstable"},
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func AudioPlay(ctx context.Context, s *testing.State) {
	l, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer func() {
		l.Close(ctx)
		if err := faillog.Save(s.HasError, l, s.OutDir()); err != nil {
			s.Log("Failed to save lacros logs: ", err)
		}
	}()

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
