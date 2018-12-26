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
	"chromiumos/tast/local/bundles/cros/arc/mediasession"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaSessionGain,
		Desc:         "Checks Android gain audio focus requests are forwarded to Chrome",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "android_p", "chrome_login"},
		Timeout:      4 * time.Minute,
		Data: []string{
			"media_session_test.apk",
			"media_session_60sec_test.ogg",
			"media_session_test.html",
		},
	})
}

func MediaSessionGain(ctx context.Context, s *testing.State) {
	const buttonStartID = "org.chromium.arc.testapp.media_session:id/button_start_test"

	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	mediasession.RunTest(ctx, s, func(a *arc.ARC, d *ui.Device, sr *httptest.Server, cr *chrome.Chrome) {
		s.Log("Clicking the start test button")
		must(d.Object(ui.ID(buttonStartID)).Click(ctx))

		s.Log("Waiting for the entries to show that we have acquired audio focus")
		must(mediasession.WaitForAndroidAudioFocusGain(ctx, d, mediasession.AudioFocusGain))

		s.Log("Launching media playback in Chrome")
		conn, err := mediasession.LoadTestPageAndStartPlaying(ctx, cr, sr)
		if err != nil {
			s.Fatal("failed to start playback: ", err)
		}
		defer conn.Close()

		s.Log("Switching to the test app")
		must(mediasession.SwitchToTestApp(ctx, a))

		s.Log("Waiting for audio focus loss")
		must(mediasession.WaitForAndroidAudioFocusGain(ctx, d, mediasession.AudioFocusLoss))

		s.Log("Clicking the start test button")
		must(d.Object(ui.ID(buttonStartID)).Click(ctx))

		s.Log("Waiting for the entries to show that we have acquired audio focus")
		must(mediasession.WaitForAndroidAudioFocusGain(ctx, d, mediasession.AudioFocusGain))

		s.Log("Checking that Chrome has lost audio focus")
		must(conn.Exec(ctx, mediasession.CheckChromeIsPaused))
	})
}
