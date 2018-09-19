// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcapp

import (
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcapp/apptest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaSession,
		Desc:         "Checks Android audio focus requests are forwarded to Chrome",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      1 * time.Minute,
		Data: []string{
			"media_session_test.apk",
			"media_session_60sec_test.ogg",
			"media_session_test.html",
		},
	})
}

func MediaSession(s *testing.State) {
	const (
		// This is a build of ArcMediaSessionTest in vendor/google_arc.
		apk = "media_session_test.apk"
		pkg = "org.chromium.arc.testapp.media_session"
		cls = "org.chromium.arc.testapp.media_session.MediaSessionActivity"

		buttonStartID  = "org.chromium.arc.testapp.media_session:id/button_start_test"
		testResultID   = "org.chromium.arc.testapp.media_session:id/test_result"
		currentFocusID = "org.chromium.arc.testapp.media_session:id/current_focus"

		// These are defined by the Android AudioManager API.
		// https://developer.android.com/reference/android/media/AudioManager
		audioFocusSuccess = "1"
		audioFocusGain    = "1"
		audioFocusLoss    = "-1"
	)

	ctx := s.Context()

	args := []string{"--enable-audio-focus", "--enable-features=ArcEnableUnifiedAudioFocus"}

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	apptest.RunWithChrome(s, cr, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		s.Log("Waiting for the default entries to show up")
		must(d.Object(ui.ID(testResultID)).WaitForExists(ctx))
		must(d.Object(ui.ID(currentFocusID)).WaitForExists(ctx))

		s.Log("Clicking the start test button")
		must(d.Object(ui.ID(buttonStartID)).Click(ctx))

		s.Log("Waiting for the entries to show that we have acquired audio focus")
		must(d.Object(ui.ID(testResultID), ui.Text(audioFocusSuccess)).WaitForExists(ctx))
		must(d.Object(ui.ID(currentFocusID), ui.Text(audioFocusGain)).WaitForExists(ctx))

		s.Log("Launching media playback in Chrome")
		conn, err := cr.NewConn(ctx, server.URL+"/media_session_test.html")
		if err != nil {
			s.Fatal("Failed to open Media Internals: ", err)
		}
		defer conn.Close()

		if err := conn.Exec(ctx, "audio.play()"); err != nil {
			s.Fatal("Failed play: ", err)
		}

		if err := conn.WaitForExpr(ctx, "audio.currentTime > 0"); err != nil {
			s.Fatal("Timed out waiting for playback: ", err)
		}

		s.Log("Switching to the test app")
		if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
			s.Fatal("Failed switching to app: ", err)
		}

		s.Log("Waiting for audio focus loss")
		must(d.Object(ui.ID(currentFocusID), ui.Text(audioFocusLoss)).WaitForExists(ctx))

		s.Log("Clicking the start test button")
		must(d.Object(ui.ID(buttonStartID)).Click(ctx))

		s.Log("Waiting for the entries to show that we have acquired audio focus")
		must(d.Object(ui.ID(testResultID), ui.Text(audioFocusSuccess)).WaitForExists(ctx))
		must(d.Object(ui.ID(currentFocusID), ui.Text(audioFocusGain)).WaitForExists(ctx))

		s.Log("Checking that Chrome has lost audio focus")
		if err := conn.WaitForExpr(ctx, "audio.paused"); err != nil {
			s.Fatal("Timed out waiting for paused: ", err)
		}
	})
}
