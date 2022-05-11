// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package a11y

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromevoxPlainTextEditing,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "A test that verifies the way ChromeVox can be used to edit text in plain text fields",
		Contacts: []string{
			"katie@chromium.org",           // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func ChromevoxPlainTextEditing(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(cleanupCtx)

	// Setup a browser before opening a new tab.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	c, err := a11y.NewTabWithHTML(ctx, br, `<label for='singleLine'>singleLine</label>
<input type='text' id='singleLine' value='Single line field'><br>
<label for='textarea'>textArea</label>
<textarea id='textarea'>Line 1
line 2
line 3</textarea>`)
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	defer c.Close()

	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	defer func() {
		if err := a11y.SetFeatureEnabled(cleanupCtx, tconn, a11y.SpokenFeedback, false); err != nil {
			s.Error("Failed to disable spoken feedback: ", err)
		}
	}()

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

	const nextObject = "Search+Right"
	const lang = "en-US"

	testSteps := []struct {
		keyCommands  []string
		expectations []a11y.SpeechExpectation
	}{
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("singleLine", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("Single line field", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("Edit text", lang, 1.0, 1.0)},
		},
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("textArea", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("Line 1 line 2 line 3", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("Text area", lang, .8, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("I")},
		},
		{
			[]string{"Shift+Right"},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("I", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("selected", lang, 1.0, 1.0)},
		},
		{
			[]string{"Down"},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("line 2")},
		},
		{
			[]string{"Left"},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("L")},
		},
		{
			[]string{"Shift+Ctrl+Right"},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("line", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("selected", lang, 1.0, 1.0)},
		},
		{
			[]string{"Shift+Ctrl+Right"},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("2", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("added to selection", lang, 1.0, 1.0)},
		},
	}

	for _, step := range testSteps {
		if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, step.keyCommands, step.expectations); err != nil {
			s.Error("Error when pressing keys and expecting speech: ", err)
		}
	}
}
