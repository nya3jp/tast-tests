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
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioPinnedStream,
		Desc:         "Tests pinned stream on lacros",
		Contacts:     []string{"yuhsuan@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosStartedByDataBypassPermissions",
		Data:         []string{"media_session_60sec_test.ogg", "audio_playback_test.html"},
		Params: []testing.Param{{
			Name: "play",
			Val:  audio.OutputStream,
		}, {
			Name: "record",
			Val:  audio.InputStream,
		}},
	})
}

func AudioPinnedStream(ctx context.Context, s *testing.State) {
	// Load ALSA loopback module.
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}
	defer unload(ctx)

	lc, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer lc.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := lc.NewConn(ctx, server.URL+"/audio_playback_test.html")
	if err != nil {
		s.Fatal(err, "failed to open new tab")
	}
	defer conn.Close()

	deviceID := func(devName string) string {
		var devID string
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conn.Call(ctx, &devID, `async function(deviceName) {
				let devices = await navigator.mediaDevices.enumerateDevices();
				return devices.find((dev) => dev.label == deviceName).deviceId;
			}`, devName); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Fatal("Failed to find loopback device: ", err)
		}
		return devID
	}

	if s.Param() == audio.InputStream {
		loopbackID := deviceID("Loopback Capture")
		if err := conn.Call(ctx, nil, `async function(loopbackId) {
			let stream = await navigator.mediaDevices.getUserMedia({ audio: {deviceId: loopbackId}, video: false })
			new MediaRecorder(stream).start();
		}`, loopbackID); err != nil {
			s.Fatalf("Failed to start recording on %s: %v", loopbackID, err)
		}
	} else {
		loopbackID := deviceID("Loopback Playback")
		if err := conn.Call(ctx, nil, `async function(loopbackId) {
			await audio.setSinkId(loopbackId);
		}`, loopbackID); err != nil {
			s.Fatalf("Failed to set sink id to %s: %v", loopbackID, err)
		}
		if err := conn.Eval(ctx, "audio.play()", nil); err != nil {
			s.Fatal("Failed to start playing: ", err)
		}
		if err := conn.WaitForExpr(ctx, "audio.currentTime > 0"); err != nil {
			s.Fatal("Failed to wait for audio to play: ", err)
		}
	}

	streams, err := crastestclient.WaitForStreams(ctx, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to wait for streams: ", err)
	}

	containsLacrosPinnedStream := func(streams []crastestclient.StreamInfo) bool {
		for _, stream := range streams {
			if stream.ClientType == "CRAS_CLIENT_TYPE_LACROS" && stream.IsPinned == true {
				return true
			}
		}
		return false
	}

	if !containsLacrosPinnedStream(streams) {
		s.Error("Failed to find the pinned stream")
	}
}
