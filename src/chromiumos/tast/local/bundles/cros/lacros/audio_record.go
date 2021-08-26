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
		Func:         AudioRecord,
		Desc:         "Tests basic audio recording on lacros",
		Contacts:     []string{"yuhsuan@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosBypassPermissions",
		Timeout:      7 * time.Minute, // A lenient limit for launching Lacros Chrome.
	})
}

func AudioRecord(ctx context.Context, s *testing.State) {
	l, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData))
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
	defer conn.Close()

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
	}
}
