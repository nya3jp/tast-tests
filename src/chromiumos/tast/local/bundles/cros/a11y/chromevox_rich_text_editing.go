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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromevoxRichTextEditing,
		LacrosStatus: testing.LacrosVariantUnneeded, // TODO(crbug.com/1159107): Test is disabled in continuous testing. Migrate when enabled.
		Desc:         "A test that verifies the way ChromeVox can be used to edit text in a contenteditable",
		Contacts: []string{
			"katie@chromium.org",           // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		// TODO(https://crbug.com/1159107): Investigate failures and re-enable this test.
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func ChromevoxRichTextEditing(ctx context.Context, s *testing.State) {
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

	c, err := a11y.NewTabWithHTML(ctx, cr.Browser(), `<div role="textbox" contenteditable>
<h2>hello</h2>
<div><br></div>
<p>This is a <a href="%23test">test</a> of rich text</p>
</div>
<div role="textbox" contenteditable>
<p style="font-size:20px; font-family:times"><b style="color:%23ff0000">Move</b>
<i>through</i> <u style="font-family:georgia">text</u> by <strike style="font-size:12px; color:%230000ff">character</strike>
<a href="%23">test</a>!</p>
</div>
<div contenteditable="true" role="textbox">
<p>Start</p>
<span>I </span><span role="suggestion" aria-description="Username">
<span role="insertion">was</span>
<span role="deletion">am</span></span><span> typing</span>
<p>End</p></div>`)
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
	const formatPitch = 0.75
	const rolePitch = 0.7
	const classPitch = 0.8

	testSteps := []struct {
		keyCommands  []string
		expectations []a11y.SpeechExpectation
	}{
		{
			[]string{nextObject},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("hello", lang, 1.0, 1.0)},
		},
		{
			[]string{"Down", "Down"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("new line"),
				a11y.NewOptionsExpectation("This is a", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("test", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("Link", lang, rolePitch, 1.0),
				a11y.NewOptionsExpectation("of rich text", lang, 1.0, 1.0)},
		},
		{
			[]string{"Up", "Up"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("new line"),
				a11y.NewStringExpectation("hello"),
				a11y.NewOptionsExpectation("Heading 2", lang, classPitch, 1.0)},
		},
		{
			[]string{"Tab"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("Move through text by character test!"),
				a11y.NewOptionsExpectation("Text area", lang, classPitch, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("O"),
				a11y.NewOptionsExpectation("Red, 100% opacity.", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Bold", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Font Tinos", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right", "Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("V"),
				a11y.NewStringExpectation("E")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("space"),
				a11y.NewOptionsExpectation("Black, 100% opacity.", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Not bold", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("T"),
				a11y.NewOptionsExpectation("Italic", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right", "Right", "Right", "Right", "Right", "Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("H"),
				a11y.NewStringExpectation("R"),
				a11y.NewStringExpectation("O"),
				a11y.NewStringExpectation("U"),
				a11y.NewStringExpectation("G"),
				a11y.NewStringExpectation("H")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("space"),
				a11y.NewOptionsExpectation("Not italic", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("T"),
				a11y.NewOptionsExpectation("Underline", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Font Georgia", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right", "Right", "Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("E"),
				a11y.NewStringExpectation("X"),
				a11y.NewStringExpectation("T")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("space"),
				a11y.NewOptionsExpectation("Not underline", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Font Tinos", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right", "Right", "Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("B"),
				a11y.NewStringExpectation("Y"),
				a11y.NewStringExpectation("space")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("C"),
				a11y.NewOptionsExpectation("Blue, 100% opacity.", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Line through", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right", "Right", "Right", "Right", "Right", "Right", "Right", "Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("H"),
				a11y.NewStringExpectation("A"),
				a11y.NewStringExpectation("R"),
				a11y.NewStringExpectation("A"),
				a11y.NewStringExpectation("C"),
				a11y.NewStringExpectation("T"),
				a11y.NewStringExpectation("E"),
				a11y.NewStringExpectation("R")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("space"),
				a11y.NewOptionsExpectation("Not line through", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("Link", lang, rolePitch, 1.0),
				a11y.NewStringExpectation("T"),
				a11y.NewOptionsExpectation("Blue, 100% opacity.", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Link", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Underline", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right", "Right", "Right"},
			[]a11y.SpeechExpectation{
				a11y.NewStringExpectation("E"),
				a11y.NewStringExpectation("S"),
				a11y.NewStringExpectation("T")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("exclamation", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("Black, 100% opacity.", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Not link", lang, formatPitch, 1.0),
				a11y.NewOptionsExpectation("Not underline", lang, formatPitch, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("End of text", lang, 1.0, 1.0)},
		},
		{
			[]string{"Tab", "Down"},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("Start")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("space", lang, 1.0, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("Suggest", lang, rolePitch, 1.0),
				a11y.NewOptionsExpectation("Username", lang, 1.0, 1.0),
				a11y.NewOptionsExpectation("Insert", lang, rolePitch, 1.0),
				a11y.NewOptionsExpectation("W", lang, 1.0, 1.0)},
		},
		{
			[]string{"Right", "Right"},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("A"), a11y.NewStringExpectation("S")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{a11y.NewOptionsExpectation("Exited Insert.", lang, classPitch, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("Delete", lang, rolePitch, 1.0),
				a11y.NewOptionsExpectation("A", lang, 1.0, 1.0)},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("M")},
		},
		{
			[]string{"Right"},
			[]a11y.SpeechExpectation{
				a11y.NewOptionsExpectation("Exited Delete.", lang, classPitch, 1.0),
				a11y.NewOptionsExpectation("Exited Suggest.", lang, classPitch, 1.0)},
		},
	}

	for _, step := range testSteps {
		if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, step.keyCommands, step.expectations); err != nil {
			s.Error("Error when pressing keys and expecting speech: ", err)
		}
	}
}
