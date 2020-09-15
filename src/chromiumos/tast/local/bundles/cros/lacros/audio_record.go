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
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioRecord,
		Desc:         "Tests basic audio recording on lacros",
		Contacts:     []string{"yuhsuan@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          launcher.StartedByData(),
		Timeout:      7 * time.Minute, // A lenient limit for launching Lacros Chrome.
		Data:         []string{launcher.DataArtifact, "audio_record_test.html"},
	})
}

func AudioRecord(ctx context.Context, s *testing.State) {
	l, err := launcher.LaunchLacrosChrome(ctx, s.PreValue().(launcher.PreData))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := l.NewConn(ctx, server.URL+"/audio_record_test.html")
	if err != nil {
		s.Fatal(err, "failed to open new tab")
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "checkAudioInput()"); err != nil {
		var msg string
		if err := conn.Eval(ctx, "enumerateDevicesError", &msg); err != nil {
			s.Error("Failed to evaluate enumerateDevicesError: ", err)
		} else if len(msg) > 0 {
			s.Error("enumerateDevices failed: ", msg)
		}
		s.Fatal("Timed out waiting for audio device to be available: ", err)
	}

	s.Log("Start recording")
	if err := conn.Eval(ctx, "startRecord()", nil); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	if _, err = audio.FirstRunningDevice(ctx, audio.InputStream); err != nil {
		s.Error("Failed to detect running input device: ", err)
	}
}
