// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	arcmedia "chromiumos/tast/local/bundles/cros/arc/mediasession"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/mediasession"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaSessionGainTransient,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks Android transient audio focus requests are forwarded to Chrome",
		Contacts:     []string{"beccahughes@chromium.org", "arc-eng@google.com"},
		// b:238260020 - disable aged (>1y) unpromoted informational tests
		// Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Data: []string{
			"media_session_test.apk",
			"media_session_60sec_test.ogg",
			"media_session_test.html",
		},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func MediaSessionGainTransient(ctx context.Context, s *testing.State) {
	const buttonStartID = "org.chromium.arc.testapp.media_session:id/button_start_test_transient"

	arcmedia.RunTest(ctx, s, func(a *arc.ARC, d *ui.Device, sr *httptest.Server, cr *chrome.Chrome) {
		s.Log("Launching media playback in Chrome")
		conn, err := mediasession.LoadTestPage(ctx, cr, sr.URL+"/media_session_test.html")
		if err != nil {
			s.Fatal("Failed to start playback: ", err)
		}
		defer conn.Close()

		if err := conn.Play(ctx); err != nil {
			s.Fatal("Failed to start playing: ", err)
		}

		s.Log("Switching to the test app")
		if err := arcmedia.SwitchToTestApp(ctx, a); err != nil {
			s.Fatal("Failed to switch to the test app: ", err)
		}

		s.Log("Clicking the start test button")
		if err := d.Object(ui.ID(buttonStartID)).Click(ctx); err != nil {
			s.Fatal("Failed to click the start button: ", err)
		}

		s.Log("Waiting for the entries to show that we have acquired audio focus")
		if err := arcmedia.WaitForAndroidAudioFocusGain(ctx, d, arcmedia.AudioFocusGainTransient); err != nil {
			s.Fatal("Failed to gain transient focus: ", err)
		}

		s.Log("Checking that Chrome has lost audio focus")
		if state, err := conn.State(ctx); err != nil {
			s.Fatal("Failed to get audio state: ", err)
		} else if state != mediasession.StatePaused {
			s.Fatalf("Unexpected audio state: got %s; want %s", state, mediasession.StatePaused)
		}

		s.Log("Clicking the abandon focus button")
		if err := arcmedia.AbandonAudioFocusInAndroid(ctx, d); err != nil {
			s.Fatal("Failed to abandon audio focus in android: ", err)
		}

		s.Log("Waiting for focus change")
		if err := arcmedia.WaitForAndroidAudioFocusChange(ctx, d, arcmedia.AudioFocusGainTransient); err != nil {
			s.Fatal("Failed to wait for audio focus change: ", err)
		}

		s.Log("Checking that Chrome is still playing")
		if state, err := conn.State(ctx); err != nil {
			s.Fatal("Failed to get audio state: ", err)
		} else if state != mediasession.StatePlaying {
			s.Fatalf("Unexpected audio state: got %s; want %s", state, mediasession.StatePlaying)
		}
	})
}
