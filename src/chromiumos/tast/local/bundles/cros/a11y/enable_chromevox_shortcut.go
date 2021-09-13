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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: EnableChromevoxShortcut,
		Desc: "A test that verifies Ctrl+Alt+Z enables Chromevox",
		Contacts: []string{
			"dtseng@chromium.org",          // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func EnableChromevoxShortcut(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()
	defer crastestclient.Unmute(ctxCleanup)

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

	ctrlAltZ := []string{"Ctrl+Alt+z"}
	expectedSpeech := []a11y.SpeechExpectation{a11y.NewRegexExpectation("ChromeVox spoken feedback is ready")}

	// Use the speech monitor to ensure that the spoken announcement was given.
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, ctrlAltZ, expectedSpeech); err != nil {
		s.Fatal("Failed to verify Chromevox enabled: ", err)
	}
}
