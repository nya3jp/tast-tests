// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"

	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromevoxNumberReadingStyle,
		Desc: "Verifies ChromeVox honors its setting to read numbers as words or as digits",
		Contacts: []string{
			"josiahk@chromium.org",         // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func ChromevoxNumberReadingStyle(ctx context.Context, s *testing.State) {
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
		s.Fatal("Failed to enable ChromeVox: ", err)
	}
	defer func() {
		if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, false); err != nil {
			s.Error("Failed to disable ChromeVox: ", err)
		}
	}()

	cvconn, err := a11y.NewChromeVoxConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to connect to the ChromeVox background page: ", err)
	}
	defer cvconn.Close()

	vd := a11y.VoiceData{
		ExtID:  a11y.GoogleTTSExtensionID,
		Locale: "en-US",
	}
	if err := cvconn.SetVoice(ctx, vd); err != nil {
		s.Fatal("Failed to set the ChromeVox voice: ", err)
	}

	if err := a11y.SetTTSRate(ctx, tconn, 5.0); err != nil {
		s.Fatal("Failed to change TTS rate: ", err)
	}
	defer a11y.SetTTSRate(ctx, tconn, 1.0)

	ed := a11y.TTSEngineData{
		ExtID:                     a11y.GoogleTTSExtensionID,
		UseOnSpeakWithAudioStream: false,
	}
	sm, err := a11y.RelevantSpeechMonitor(ctx, cr, tconn, ed)
	if err != nil {
		s.Fatal("Failed to connect to the TTS background page: ", err)
	}
	defer sm.Close()

	// Wait for ChromeVox to focus the root web area.
	rootWebArea := nodewith.Role(role.RootWebArea).First()
	if err = cvconn.WaitForFocusedNode(ctx, tconn, rootWebArea); err != nil {
		s.Error("Failed to wait for initial ChromeVox focus: ", err)
	}

	testSteps := []struct {
		KeyCommands  []string
		Expectations []a11y.SpeechExpectation
	}{
		{
			[]string{cvox.OpenOptionsOne, cvox.OpenOptionsTwo},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("ChromeVox Options")},
		},
		{
			[]string{cvox.Find},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Find")},
		},
		{
			[]string{"R", "E", "A", "D", cvox.Space, "N", "U", "M", "B", "E", "R", "S", cvox.Escape},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Read numbers as:")},
		},
		{
			[]string{cvox.NextObject},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Words")},
		},
		{
			[]string{cvox.Activate},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Expanded")},
		},
		{
			[]string{cvox.ArrowDown},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Digits")},
		},
		{
			[]string{cvox.Activate},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Read numbers as:"), a11y.NewStringExpectation("Digits")},
		},
	}

	for _, step := range testSteps {
		if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, step.KeyCommands, step.Expectations); err != nil {
			s.Error("Error when pressing keys and expecting speech: ", err)
		}
	}
}
