// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioRecord,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests basic audio recording on lacros",
		Contacts:     []string{"yuhsuan@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosAudio",
		Timeout:      7 * time.Minute, // A lenient limit for launching Lacros Chrome.
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable", "audio_stable"},
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros_unstable"},
		}},
	})
}

func AudioRecord(ctx context.Context, s *testing.State) {
	chrome := s.FixtValue().(chrome.HasChrome).Chrome()

	// Load ALSA loopback module.
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}
	defer unload(ctx)

	if err = audio.SetupLoopback(ctx, chrome); err != nil {
		s.Fatal("Failed to setup loopback device: ", err)
	}

	tconn, err := chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	l, err := lacros.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := l.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(err, "failed to open new tab")
	}

	var found bool
	s.Log("Check audio input")
	if err := conn.Call(ctx, &found, `async () => {
		let devices = await navigator.mediaDevices.enumerateDevices();
		return devices.some((dev) => dev.kind == 'audioinput');
	}`); err != nil {
		s.Fatal("Failed to check audio input: ", err)
	}

	if !found {
		s.Fatal("Failed to find audio input devices")
	}

	s.Log("Start recording")
	if err := conn.Call(ctx, nil, `async () => {
		let stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false })
		new MediaRecorder(stream).start();
	}`); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	if _, err = crastestclient.FirstRunningDevice(ctx, audio.InputStream); err != nil {
		s.Error("Failed to detect running input device: ", err)
		if err := crastestclient.DumpAudioDiagnostics(ctx, s.OutDir()); err != nil {
			s.Error("Failed to dump audio diagnostics: ", err)
		}
	}
}
