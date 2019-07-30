// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	arcmedia "chromiumos/tast/local/bundles/cros/arc/mediasession"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/mediasession"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaSessionGainTransientDuck,
		Desc:         "Checks Android ducking audio focus requests are forwarded to Chrome",
		Contacts:     []string{"beccahughes@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Data: []string{
			"media_session_test.apk",
			"media_session_60sec_test.ogg",
			"media_session_test.html",
		},
	})
}

func MediaSessionGainTransientDuck(ctx context.Context, s *testing.State) {
	const (
		buttonStartID = "org.chromium.arc.testapp.media_session:id/button_start_test_duck"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	arcmedia.RunTest(ctx, s, func(a *arc.ARC, d *ui.Device, sr *httptest.Server, cr *chrome.Chrome) {
		s.Log("Launching media playback in Chrome")
		conn, err := mediasession.LoadTestPageAndStartPlaying(ctx, cr, sr.URL+"/media_session_test.html")
		if err != nil {
			s.Fatal("Failed to start playback: ", err)
		}
		defer conn.Close()

		s.Log("Switching to the test app")
		must(arcmedia.SwitchToTestApp(ctx, a))

		s.Log("Clicking the start test button")
		must(d.Object(ui.ID(buttonStartID)).Click(ctx))

		s.Log("Waiting for the entries to show that we have acquired audio focus")
		must(arcmedia.WaitForAndroidAudioFocusGain(ctx, d, arcmedia.AudioFocusGainTransientMayDuck))

		s.Log("Checking that Chrome has not lost audio focus")
		must(conn.Exec(ctx, mediasession.CheckChromeIsPlaying))

		s.Log("Clicking the abandon focus button")
		must(arcmedia.AbandonAudioFocusInAndroid(ctx, d))

		s.Log("Checking that Chrome is still playing")
		must(conn.Exec(ctx, mediasession.CheckChromeIsPlaying))
	})
}
