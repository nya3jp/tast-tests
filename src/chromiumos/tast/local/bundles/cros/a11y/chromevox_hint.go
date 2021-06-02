// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"
	"time"

	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromevoxHint,
		Desc: "A test that verifies the behavior of the ChromeVox hint in OOBE. This is a feature that is activated after 20s of idle on the OOBE welcome page. After 20s of idle, we show a dialog and give a spoken announcement with instructions for activating ChromeVox",
		Contacts: []string{
			"akihiroota@chromium.org",      // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func ChromevoxHint(ctx context.Context, s *testing.State) {
	// This feature is disabled in dev mode, so pass in the flag to explicitly enable it.
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")), chrome.ExtraArgs("--enable-oobe-chromevox-hint-timer-for-dev-mode"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	// Wait for the welcome screen to be shown.
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}

	// Get a speech monitor for the Google TTS engine.
	ed := a11y.TTSEngineData{
		ExtID:                     a11y.GoogleTTSExtensionID,
		UseOnSpeakWithAudioStream: false,
	}

	sm, err := a11y.GetRelevantSpeechMonitor(ctx, cr, tconn, ed)
	if err != nil {
		s.Fatal("Failed to connect to the TTS background page: ", err)
	}
	defer sm.Close()

	// Wait for the ChromeVox hint dialog to be shown on-screen.
	// This should only take 20s from when idle first begins, but allow 30s to
	// avoid any potential race conditions.
	chromeVoxDialog, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Name:  "Do you want to activate ChromeVox, the built-in screen reader for Chrome OS?",
		State: map[ui.StateType]bool{ui.StateTypeOffscreen: false},
	}, 30*time.Second)
	if err != nil {
		s.Fatal("Failed to find the ChromeVox hint dialog: ", err)
	}
	defer chromeVoxDialog.Release(ctx)

	// Use the speech monitor to ensure that the spoken announcement was given.
	err = sm.Consume(ctx, []string{"Do you want to activate ChromeVox, the built-in screen reader for Chrome OS? If so, press the space bar."})
	if err != nil {
		s.Fatal("Failed to verify the ChromeVox hint announcement: ", err)
	}

	// Press Escape to close the dialog.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	if err := ew.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Error when pressing Escape: ", err)
	}
}
