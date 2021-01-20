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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SpokenFeedback,
		Desc: "A spoken feedback test that executes commands and verifies speech output",
		Contacts: []string{
			"akihiroota@chromium.org", // Test author
			"tast-users@chromium.org", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func SpokenFeedback(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer tconn.Close()

	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	defer func() {
		if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, false); err != nil {
			s.Fatal("Failed to disable spoken feedback: ", err)
		}
	}()

	// Get connection to the ChromeVox background page.
	cvconn, err := a11y.NewChromeVoxConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to connect to the ChromeVox background page: ", err)
	}
	defer cvconn.Close()

	// Get connection to the Google TTS background page.
	tts, err := a11y.NewGoogleTtsConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to connect to the Google TTS background page: ", err)
	}
	defer tts.Close()

	// Open the ChromeVox options page and verify speech output.
	if err := cvconn.DoCommand(ctx, "showOptionsPage"); err != nil {
		s.Fatal("Failed to perform the showOptionsPage ChromeVox command: ", err)
	}
	if err := tts.ExpectSpeech(ctx, []string{"tab created", "ChromeVox Options"}); err != nil {
		s.Fatal("Failed to verify speech from the Google TTS engine: ", err)
	}

	// Open a connection to the keyboard.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	// Press Alt+Shift+L, which moves ChromeVox focus to the launcher.
	if err := ew.Accel(ctx, "Alt+Shift+L"); err != nil {
		s.Fatal("Error when pressing Alt+Shift+L: ")
	}
	if err = tts.ExpectSpeech(ctx, []string{"Launcher", "Button", "Shelf", "Tool bar"}); err != nil {
		s.Fatal("Failed to verify speech from the Google TTS engine: ", err)
	}

	// Press Alt+Shift+S, which moves ChromeVox focus to the shelf.
	if err = ew.Accel(ctx, "Alt+Shift+S"); err != nil {
		s.Fatal("Error when pressing Alt+Shift+S: ")
	}
	if err = tts.ExpectSpeech(ctx, []string{"Quick Settings, Press search plus left to access the notification center., window"}); err != nil {
		s.Fatal("Failed to verify speech from the Google TTS engine: ", err)
	}
}
