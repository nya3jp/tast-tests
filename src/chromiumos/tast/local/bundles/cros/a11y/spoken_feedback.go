// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"
	"time"

	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// Constants for keyboard shortcuts.
const (
	nextObject       = "Search+Right"
	previousObject   = "Search+Left"
	jumpToLauncher   = "Alt+Shift+L"
	jumpToStatusTray = "Alt+Shift+S"
)

type spokenFeedbackTestData struct {
	VoiceData  a11y.VoiceData
	EngineData a11y.TTSEngineData
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SpokenFeedback,
		Desc: "A spoken feedback test that executes ChromeVox commands and keyboard shortcuts, and verifies that correct speech is given by the Google and eSpeak TTS engines",
		Contacts: []string{
			"akihiroota@chromium.org",      // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "google_tts",
			Val: spokenFeedbackTestData{
				VoiceData: a11y.VoiceData{
					ExtID:  a11y.GoogleTTSExtensionID,
					Locale: "en-US",
				},
				EngineData: a11y.TTSEngineData{
					ExtID:                     a11y.GoogleTTSExtensionID,
					UseOnSpeakWithAudioStream: false,
				},
			},
		}, {
			Name: "espeak",
			Val: spokenFeedbackTestData{
				VoiceData: a11y.VoiceData{
					// eSpeak does not come with an English voice built-in, so we need to
					// use another language. We use Greek here since the voice is built-in
					// and capable of speaking English words.
					ExtID:  a11y.ESpeakExtensionID,
					Locale: "el",
				},
				EngineData: a11y.TTSEngineData{
					ExtID:                     a11y.ESpeakExtensionID,
					UseOnSpeakWithAudioStream: true,
				},
			},
		}},
	})
}

func SpokenFeedback(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(ctx)

	c, err := a11y.NewTabWithHTML(ctx, cr, "<p>Start</p><p>This is a ChromeVox test</p><p>End</p>")
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	defer c.Close()

	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	defer func() {
		if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, false); err != nil {
			s.Error("Failed to disable spoken feedback: ", err)
		}
	}()

	cvconn, err := a11y.NewChromeVoxConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to connect to the ChromeVox background page: ", err)
	}
	defer cvconn.Close()

	td := s.Param().(spokenFeedbackTestData)
	vd := td.VoiceData
	ed := td.EngineData
	if err := cvconn.SetVoice(ctx, vd); err != nil {
		s.Fatal("Failed to set the ChromeVox voice: ", err)
	}

	if err := a11y.SetTTSRate(ctx, tconn, 5.0); err != nil {
		s.Fatal("Failed to change TTS rate: ", err)
	}
	defer a11y.SetTTSRate(ctx, tconn, 1.0)

	sm, err := a11y.RelevantSpeechMonitor(ctx, cr, tconn, ed)
	if err != nil {
		s.Fatal("Failed to connect to the TTS background page: ", err)
	}
	defer sm.Close()

	// Wait for ChromeVox to focus the root web area.
	rootWebArea := nodewith.Role(role.RootWebArea).First()
	if err = cvconn.WaitForFocusedNode(ctx, tconn, rootWebArea, 10*time.Second); err != nil {
		s.Error("Failed to wait for initial ChromeVox focus: ", err)
	}

	// Test basic navigation.
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{nextObject}, []a11y.SpeechExpectation{a11y.NewStringExpectation("Start")}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}

	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{nextObject}, []a11y.SpeechExpectation{a11y.NewStringExpectation("This is a ChromeVox test")}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}

	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{nextObject}, []a11y.SpeechExpectation{a11y.NewStringExpectation("End")}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}

	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{previousObject}, []a11y.SpeechExpectation{a11y.NewStringExpectation("This is a ChromeVox test")}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}

	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{previousObject}, []a11y.SpeechExpectation{a11y.NewStringExpectation("Start")}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}

	// Test system-wide shortcuts.
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{jumpToLauncher},
		[]a11y.SpeechExpectation{
			a11y.NewStringExpectation("Launcher"),
			a11y.NewStringExpectation("Button"),
			a11y.NewStringExpectation("Shelf"),
			a11y.NewStringExpectation("Tool bar"),
			a11y.NewStringExpectation("Press Search plus Space to activate"),
		}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{jumpToStatusTray},
		[]a11y.SpeechExpectation{
			a11y.NewRegexExpectation("Quick Settings*"),
			a11y.NewRegexExpectation("Press search plus left to access the notification center*"),
			a11y.NewStringExpectation("window"),
		}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}
}
