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
		Func: SelectToSpeak,
		Desc: "A test that invokes Select-to-Speak and verifies the correct speech is given by the Google TTS engine",
		Contacts: []string{
			"akihiroota@chromium.org",      // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func SelectToSpeak(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(cleanupCtx)

	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SelectToSpeak, true); err != nil {
		s.Fatal("Failed to enable Select-to-Speak: ", err)
	}
	defer func(ctx context.Context) {
		if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SelectToSpeak, false); err != nil {
			s.Error("Failed to disable Select-to-Speak: ", err)
		}
	}(cleanupCtx)

	ed := a11y.TTSEngineData{ExtID: a11y.GoogleTTSExtensionID, UseOnSpeakWithAudioStream: false}
	sm, err := a11y.RelevantSpeechMonitor(ctx, cr, tconn, ed)
	if err != nil {
		s.Fatal("Failed to connect to the Google TTS background page: ", err)
	}
	defer sm.Close()

	c, err := a11y.NewTabWithHTML(ctx, cr, "<p>This is a select-to-speak test</p>")
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	c.Close()

	// Select all and invoke Select-to-Speak.
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{"Ctrl+A", "Search+S"}, []a11y.SpeechExpectation{a11y.NewStringExpectation("This is a select-to-speak test")}); err != nil {
		s.Error("Error when pressing keys and expecting speech: ", err)
	}
}
