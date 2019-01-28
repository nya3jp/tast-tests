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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// AudioFocusType contains the various audio focus types an Android app can have.
type AudioFocusType string

const (
	// This is a build of ArcMediaSessionTest in vendor/google_arc.
	packageName  = "org.chromium.arc.testapp.media_session"
	activityName = "org.chromium.arc.testapp.media_session.MediaSessionActivity"

	testResultID   = "org.chromium.arc.testapp.media_session:id/test_result"
	currentFocusID = "org.chromium.arc.testapp.media_session:id/current_focus"

	abandonFocusID = "org.chromium.arc.testapp.media_session:id/button_abandon"

	// The following are defined by the Android AudioManager API.
	// https://developer.android.com/reference/android/media/AudioManager

	// audioFocusSuccess is the result code when an audio focus is successful.
	audioFocusSuccess = "1"

	// AudioFocusLoss is the audio focus state when focus has been lost.
	AudioFocusLoss AudioFocusType = "-2"

	// AudioFocusGain is the audio focus state when the "Gain" audio focus type has been granted.
	AudioFocusGain AudioFocusType = "1"

	// AudioFocusGainTransient is the audio focus state when the "Gain Transient" audio focus type has been granted.
	AudioFocusGainTransient AudioFocusType = "2"

	// AudioFocusGainTransientMayDuck is the audio focus state when the "Gain Transient May Duck" audio focus type has been granted.
	AudioFocusGainTransientMayDuck AudioFocusType = "3"

	// CheckChromeIsPlaying is a JS expression that can be evaluated in the connection returned by LoadTestPageAndStartPlaying to check if the audio element on the test page is playing.
	CheckChromeIsPlaying = "!audio.paused"

	// CheckChromeIsPaused is a JS expression that can be evaluated in the connection returned by LoadTestPageAndStartPlaying to check if the audio element on the test page is paused.
	CheckChromeIsPaused = "audio.paused"
)

// TestFunc contains the contents of the test itself and is called when the browser and test app are setup
// and ready for use.
type TestFunc func(*arc.ARC, *ui.Device, *httptest.Server, *chrome.Chrome)

// SwitchToTestApp switches the focus to the test app.
func SwitchToTestApp(ctx context.Context, a *arc.ARC) error {
	return a.Command("am", "start", "-W", packageName+"/"+activityName).Run(ctx)
}

// WaitForAndroidAudioFocusGain waits for the test app to display a certain audio focus result.
func WaitForAndroidAudioFocusGain(ctx context.Context, d *ui.Device, focusType AudioFocusType) error {
	if err := d.Object(ui.ID(testResultID), ui.Text(audioFocusSuccess)).WaitForExists(ctx); err != nil {
		return err
	}

	return d.Object(ui.ID(currentFocusID), ui.Text(string(focusType))).WaitForExists(ctx)
}

// AbandonAudioFocusInAndroid tells the test app to abandon audio focus.
func AbandonAudioFocusInAndroid(ctx context.Context, dev *ui.Device) error {
	return dev.Object(ui.ID(abandonFocusID)).Click(ctx)
}

// LoadTestPageAndStartPlaying starts the media session test page in Chrome and checks that it
// has successfully started playing.
func LoadTestPageAndStartPlaying(ctx context.Context, cr *chrome.Chrome, sr *httptest.Server) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, sr.URL+"/media_session_test.html")
	if err != nil {
		return nil, err
	}

	if err := conn.Exec(ctx, "audio.play()"); err != nil {
		conn.Close()
		return nil, err
	}

	if err := conn.WaitForExpr(ctx, "audio.currentTime > 0"); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// RunTest starts Chrome with the media session features enabled. It installs the ARC test app, launches it and waits for it to be ready.
func RunTest(ctx context.Context, s *testing.State, f TestFunc) {
	const apk = "media_session_test.apk"

	args := []string{"--enable-features=ArcEnableUnifiedAudioFocus,MediaSessionService,AudioFocusEnforcement"}

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	s.Log("Starting app")

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command("am", "start", "-W", packageName+"/"+activityName).Run(ctx); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	s.Log("Waiting for the default entries to show up")

	if err := d.Object(ui.ID(testResultID)).WaitForExists(ctx); err != nil {
		s.Fatal("Failed to wait for test result text box to appear: ", err)
	}

	if err := d.Object(ui.ID(currentFocusID)).WaitForExists(ctx); err != nil {
		s.Fatal("Failed to wait for current focus text box to appear: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	f(a, d, server, cr)
}
