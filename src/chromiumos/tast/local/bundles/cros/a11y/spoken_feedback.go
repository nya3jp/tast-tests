// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"

	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SpokenFeedback,
		Desc: "A spoken feedback test that executes ChromeVox commands and keyboard shortcuts, and verifies that correct speech is given by the Google and eSpeak TTS engines",
		Contacts: []string{
			"akihiroota@chromium.org",      // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "google_tts",
			Val: a11y.VoiceData{
				ExtID:  a11y.GoogleTTSExtensionID,
				Locale: "en-US",
			},
		}, {
			Name: "espeak",
			Val: a11y.VoiceData{
				// eSpeak does not come with an English voice built-in, so we need to
				// use another language. We use Greek here since the voice is built-in
				// and capable of speaking English words.
				ExtID:  a11y.ESpeakExtensionID,
				Locale: "el",
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

	vd := s.Param().(a11y.VoiceData)
	if err := cvconn.SetVoice(ctx, vd); err != nil {
		s.Fatal("Failed to set the ChromeVox voice: ", err)
	}

	if err := a11y.SetTtsRate(ctx, tconn, 5.0); err != nil {
		s.Fatal("Faild to change tts rate: ", err)
	}
	defer a11y.SetTtsRate(ctx, tconn, 1.0)

	sm, err := a11y.NewSpeechMonitor(ctx, cr, vd.ExtID)
	if err != nil {
		s.Fatal("Failed to connect to the TTS background page: ", err)
	}
	defer sm.Close()

	if err := a11y.PressKeysAndConsumeUtterances(ctx, sm, []string{"Search+O", "O"}, []string{"ChromeVox Options"}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}

	if err := a11y.PressKeysAndConsumeUtterances(ctx, sm, []string{"Alt+Shift+L"}, []string{"Launcher", "Button", "Shelf", "Tool bar", "Press Search plus Space to activate"}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}
	if err := a11y.PressKeysAndConsumeUtterances(ctx, sm, []string{"Alt+Shift+S"}, []string{"Quick Settings,", "Press search plus left to access the notification center.,", "window"}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}
}
