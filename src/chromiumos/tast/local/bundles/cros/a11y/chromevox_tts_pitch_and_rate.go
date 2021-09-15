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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromevoxTtsPitchAndRate,
		Desc: "A test that verifies the way ChromeVox sets Text-to-Speech pitch and rate",
		Contacts: []string{
			"katie@chromium.org",           // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func ChromevoxTtsPitchAndRate(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(cleanupCtx)

	c, err := a11y.NewTabWithHTML(ctx, cr, `<p>hi</p>
		<p>high</p>
		<p>normal</p>
		<p>low</p>
		<p>fast</p>
		<p>normal</p>
		<p>slow</p>
		<textarea value="text"></textarea>
		<p>goodbye</p>`)
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	defer c.Close()

	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	defer func(ctx context.Context) {
		if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, false); err != nil {
			s.Error("Failed to disable spoken feedback: ", err)
		}
	}(cleanupCtx)

	// Connect to ChromeVox.
	cvconn, err := a11y.NewChromeVoxConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to connect to the ChromeVox background page: ", err)
	}
	defer cvconn.Close()

	// Get a speech monitor for the Google TTS engine.
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

	nextObject := "Search+Right"
	increasePitch := "Search+]"
	decreasePitch := "Search+Shift+]"
	increaseRate := "Search+["
	decreaseRate := "Search+Shift+["
	resetTtsSettings := "Search+Shift+Ctrl+\\"
	lang := "en-US"

	testSteps := []struct {
		KeyCommands  []string
		Expectations []a11y.SpeechExpectation
	}{
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("hi", lang, 1.0, 1.0)},
		},
		// Pitch is lowered for announcements.
		{
			[]string{increasePitch, increasePitch},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("Pitch 50 percent", lang, .85, 1.0),
				a11y.NewOptionsExpectation("Pitch 56 percent", lang, .95, 1.0)},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("high", lang, 1.2, 1.0)},
		},
		{
			[]string{decreasePitch, decreasePitch},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("Pitch 50 percent", lang, .85, 1.0),
				a11y.NewOptionsExpectation("Pitch 44 percent", lang, .75, 1.0)},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("normal", lang, 1, 1.0)},
		},
		{
			[]string{decreasePitch},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("Pitch 39 percent", lang, .65, 1.0)},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("low", lang, .9, 1.0)},
		},
		{
			[]string{increaseRate, increaseRate},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("Rate 19 percent", lang, .65, 1.1),
				a11y.NewOptionsExpectation("Rate 21 percent", lang, .65, 1.2)},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("fast", lang, .9, 1.2)},
		},
		{
			[]string{decreaseRate, decreaseRate},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("Rate 19 percent", lang, .65, 1.1),
				a11y.NewOptionsExpectation("Rate 17 percent", lang, .65, 1)},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("normal", lang, .9, 1)},
		},
		{
			[]string{decreaseRate},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("Rate 15 percent", lang, .65, .9)},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("slow", lang, .9, .9)},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("Text area", lang, .7, .9)},
		},
		{
			[]string{"c", "a", "t"},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("C", lang, .9, .9),
				a11y.NewOptionsExpectation("A", lang, .9, .9),
				a11y.NewOptionsExpectation("T", lang, .9, .9)},
		},
		// Pitch is lowered to delete characters.
		{
			[]string{"Backspace"},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("T", lang, .3, .9)},
		},
		{
			[]string{"Backspace"},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("A", lang, .3, .9)},
		},
		{
			[]string{resetTtsSettings},
			[]a11y.SpeechExpectation{a11y.NewRegexExpectation("Reset text to speech settings*")},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("goodbye", lang, 1, 1)},
		},
	}
	for _, step := range testSteps {
		if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, step.KeyCommands, step.Expectations); err != nil {
			s.Error("Error when pressing keys and expecting speech: ", err)
		}
	}
}
