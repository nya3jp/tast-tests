// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/a11y/chromevox"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type testParam struct {
	testData    chromevox.VoiceData
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Chromevox,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "A spoken feedback test that executes ChromeVox commands and keyboard shortcuts, and verifies that correct speech is given by the Google and eSpeak TTS engines",
		Contacts: []string{
			"akihiroota@chromium.org",      // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "google_tts",
			Fixture: "chromeLoggedIn",
			Val: testParam{
				testData: chromevox.VoiceData{
					VoiceData: a11y.VoiceData{
						ExtID:  a11y.GoogleTTSExtensionID,
						Locale: "en-US",
					},
					EngineData: a11y.TTSEngineData{
						ExtID:                     a11y.GoogleTTSExtensionID,
						UseOnSpeakWithAudioStream: false,
					},
				},
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "espeak",
			Fixture: "chromeLoggedIn",
			Val: testParam{
				testData: chromevox.VoiceData{
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
				browserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_google_tts",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: testParam{
				testData: chromevox.VoiceData{
					VoiceData: a11y.VoiceData{
						ExtID:  a11y.GoogleTTSExtensionID,
						Locale: "en-US",
					},
					EngineData: a11y.TTSEngineData{
						ExtID:                     a11y.GoogleTTSExtensionID,
						UseOnSpeakWithAudioStream: false,
					},
				},
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_espeak",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: testParam{
				testData: chromevox.VoiceData{
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
				browserType: browser.TypeLacros,
			},
		}},
	})
}

func Chromevox(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	// tconn for setting a11y features,
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()
	defer crastestclient.Unmute(ctxCleanup)

	// Setup a browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(testParam).browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	c, err := a11y.NewTabWithHTML(ctx, br, "<p>Start</p><p>This is a ChromeVox test</p><p>End</p>")
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	defer c.Close()

	// Close any existing blank tabs:
	if err := br.CloseWithURL(ctx, chrome.NewTabURL); err != nil {
		s.Fatal("Failed to close blank tab: ", err)
	}

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

	td := s.Param().(testParam).testData
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
	if err = cvconn.WaitForFocusedNode(ctx, tconn, rootWebArea); err != nil {
		s.Error("Failed to wait for initial ChromeVox focus: ", err)
	}

	testSteps := []struct {
		KeyCommands  []string
		Expectations []a11y.SpeechExpectation
	}{
		{
			[]string{chromevox.NextObject},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Start")},
		},
		{
			[]string{chromevox.NextObject},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("This is a ChromeVox test")},
		},
		{
			[]string{chromevox.NextObject},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("End")},
		},
		{
			[]string{chromevox.PreviousObject},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("This is a ChromeVox test")},
		},
		{
			[]string{chromevox.PreviousObject},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Start")},
		},
		{
			[]string{chromevox.JumpToLauncher},
			[]a11y.SpeechExpectation{a11y.NewRegexExpectation("(Launcher|Back)")},
		},
		{
			[]string{chromevox.JumpToStatusTray},
			[]a11y.SpeechExpectation{a11y.NewRegexExpectation("Quick Settings*")},
		},
	}

	for _, step := range testSteps {
		if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, step.KeyCommands, step.Expectations); err != nil {
			s.Error("Error when pressing keys and expecting speech: ", err)
		}
	}
}
