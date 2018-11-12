// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mediasession contains common utilities to help writing ARC media session tests.
package mediasession

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcapp/apptest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// This is a build of ArcMediaSessionTest in vendor/google_arc.
	apk = "media_session_test.apk"
	pkg = "org.chromium.arc.testapp.media_session"
	cls = "org.chromium.arc.testapp.media_session.MediaSessionActivity"

	testResultID   = "org.chromium.arc.testapp.media_session:id/test_result"
	currentFocusID = "org.chromium.arc.testapp.media_session:id/current_focus"

	abandonFocusID = "org.chromium.arc.testapp.media_session:id/button_abandon"

	// The following are defined by the Android AudioManager API.
	// https://developer.android.com/reference/android/media/AudioManager

	// AudioFocusSuccess is the result code when an audio focus is successful.
	AudioFocusSuccess = "1"

	// AudioFocusLoss is the audio focus state when focus has been lost.
	AudioFocusLoss = "-2"

	// AudioFocusGain is the audio focus state when the "Gain" audio focus type has been granted.
	AudioFocusGain = "1"

	// AudioFocusGainTransient is the audio focus state when the "Gain Transient" audio focus type has been granted.
	AudioFocusGainTransient = "2"

	// AudioFocusGainTransientMayDuck is the audio focus state when the "Gain Transient May Duck" audio focus type has been granted.
	AudioFocusGainTransientMayDuck = "3"
)

type testFunc func(a *arc.ARC, d *ui.Device, sr *httptest.Server, cr *chrome.Chrome)

func must(s *testing.State, err error) {
	if err != nil {
		s.Fatal(err)
	}
}

// SwitchToTestApp will switch the focus to the test app.
func SwitchToTestApp(ctx context.Context, a *arc.ARC) error {
	return a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run()
}

// CheckChromeIsPlaying checks that the media on the test page is playing. This requires the test page to have
// been opened previously.
func CheckChromeIsPlaying(ctx context.Context, cn *chrome.Conn) error {
	return cn.Exec(ctx, "!audio.paused")
}

// CheckChromeIsPaused checks that the media on the test page has been paused. This requires the test page to have
// been opened previously.
func CheckChromeIsPaused(ctx context.Context, cn *chrome.Conn) error {
	return cn.Exec(ctx, "audio.paused")
}

// WaitForAndroidAudioFocusGain waits for the test app to display a certain audio focus result.
func WaitForAndroidAudioFocusGain(ctx context.Context, d *ui.Device, focusType string) error {
	if err := d.Object(ui.ID(testResultID), ui.Text(AudioFocusSuccess)).WaitForExists(ctx); err != nil {
		return err
	}

	if err := d.Object(ui.ID(currentFocusID), ui.Text(focusType)).WaitForExists(ctx); err != nil {
		return err
	}

	return nil
}

// AbandonAudioFocusInAndroid tells the test app to abandon audio focus.
func AbandonAudioFocusInAndroid(ctx context.Context, d *ui.Device) error {
	return d.Object(ui.ID(abandonFocusID)).Click(ctx)
}

// LoadTestPageAndStartPlaying starts the media session test page in Chrome and checks that it
// has successfully started playing.
func LoadTestPageAndStartPlaying(ctx context.Context, cr *chrome.Chrome, sr *httptest.Server) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, sr.URL+"/media_session_test.html")
	if err != nil {
		return nil, err
	}

	if err := conn.Exec(ctx, "audio.play()"); err != nil {
		return nil, err
	}

	if err := conn.WaitForExpr(ctx, "audio.currentTime > 0"); err != nil {
		return nil, err
	}
	return conn, nil
}

// RunMediaSessionTest starts Chrome with the media session features enabled. It installs the ARC
// test app, launches it and waits for it to be ready.
func RunMediaSessionTest(ctx context.Context, s *testing.State, f testFunc) {
	args := []string{"--enable-audio-focus", "--enable-features=ArcEnableUnifiedAudioFocus"}

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	apptest.RunWithChrome(ctx, s, cr, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		s.Log("Waiting for the default entries to show up")
		must(s, d.Object(ui.ID(testResultID)).WaitForExists(ctx))
		must(s, d.Object(ui.ID(currentFocusID)).WaitForExists(ctx))

		f(a, d, server, cr)
	})
}
