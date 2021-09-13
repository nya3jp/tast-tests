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
		Func: ChromevoxTTSProcessingCharacters,
		Desc: "A test that verifies the way ChromeVox processes some characters for speech",
		Contacts: []string{
			"katie@chromium.org",           // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func ChromevoxTTSProcessingCharacters(ctx context.Context, s *testing.State) {
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

	c, err := a11y.NewTabWithHTML(ctx, cr, `<p>.</p>
		<p>x.</p
		<p>=========</p>
		<p>&bull; &bull;&bull;</p>
		<p>&bull;&bull;&bull;</p>
		<p>C++</p><p>C+++</p>
		<p>&pound; and %23 symbol</p>
		<p>&pound;&pound;&pound;</p>
		<p>C--</p>`)
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

	testSteps := []struct {
		expectations []a11y.SpeechExpectation
	}{
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("dot")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("x.")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("9 equal signs")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("bullet bullet bullet")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("3 bullets")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("C plus plus")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("C 3 plus signs")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("pound sterling and pound symbol")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("pound sterling pound sterling pound sterling")},
		},
		{
			[]a11y.SpeechExpectation{a11y.NewStringExpectation("C--")},
		},
	}

	for _, step := range testSteps {
		if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{"Search+Right"}, step.expectations); err != nil {
			s.Error("Error when pressing keys and expecting speech: ", err)
		}
	}
}
