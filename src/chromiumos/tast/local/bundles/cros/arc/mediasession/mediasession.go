// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mediasession contains common utilities to help writing ARC media session tests.
package mediasession

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

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
	AudioFocusLoss AudioFocusType = "-1"

	// AudioFocusGain is the audio focus state when the "Gain" audio focus type has been granted.
	AudioFocusGain AudioFocusType = "1"

	// AudioFocusGainTransient is the audio focus state when the "Gain Transient" audio focus type has been granted.
	AudioFocusGainTransient AudioFocusType = "2"

	// AudioFocusGainTransientMayDuck is the audio focus state when the "Gain Transient May Duck" audio focus type has been granted.
	AudioFocusGainTransientMayDuck AudioFocusType = "3"
)

// TestFunc contains the contents of the test itself and is called when the browser and test app are setup
// and ready for use.
type TestFunc func(*arc.ARC, *ui.Device, *httptest.Server, *chrome.Chrome)

// SwitchToTestApp switches the focus to the test app.
func SwitchToTestApp(ctx context.Context, a *arc.ARC) error {
	return a.Command(ctx, "am", "start", "-W", packageName+"/"+activityName).Run()
}

// WaitForAndroidAudioFocusGain waits for the test app to gain a certain audio focus type and display that it is successful.
func WaitForAndroidAudioFocusGain(ctx context.Context, d *ui.Device, focusType AudioFocusType) error {
	if err := d.Object(ui.ID(testResultID), ui.Text(audioFocusSuccess)).WaitForExists(ctx, 30*time.Second); err != nil {
		return err
	}

	return WaitForAndroidAudioFocusChange(ctx, d, focusType)
}

// WaitForAndroidAudioFocusChange waits for the test app to display that its audio focus type changed.
func WaitForAndroidAudioFocusChange(ctx context.Context, d *ui.Device, focusType AudioFocusType) error {
	return d.Object(ui.ID(currentFocusID), ui.Text(string(focusType))).WaitForExists(ctx, 30*time.Second)
}

// AbandonAudioFocusInAndroid tells the test app to abandon audio focus.
func AbandonAudioFocusInAndroid(ctx context.Context, dev *ui.Device) error {
	return dev.Object(ui.ID(abandonFocusID)).Click(ctx)
}

// RunTest starts Chrome with the media session features enabled. It installs the ARC test app, launches it and waits for it to be ready.
func RunTest(ctx context.Context, s *testing.State, f TestFunc) {
	extraArgs := s.Param().([]string)
	args := []string{"--enable-features=ArcEnableUnifiedAudioFocus,MediaSessionService,AudioFocusEnforcement"}
	args = append(args, extraArgs...)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const apk = "media_session_test.apk"
	s.Log("Installing and starting ", apk)
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", packageName+"/"+activityName).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	s.Log("Waiting for the default entries to show up")

	if err := d.Object(ui.ID(testResultID)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for test result text box to appear: ", err)
	}

	if err := d.Object(ui.ID(currentFocusID)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for current focus text box to appear: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	f(a, d, server, cr)
}
