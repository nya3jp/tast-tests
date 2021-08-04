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
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
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

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(ctx)

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

	sm, err := a11y.RelevantSpeechMonitor(ctx, cr, tconn, ed)
	if err != nil {
		s.Fatal("Failed to connect to the TTS background page: ", err)
	}
	defer sm.Close()

	// Wait for the ChromeVox hint dialog to be shown on-screen.
	// Detect this by waiting for the presence of the dialog's static text,
	// since the dialog itself has no name.
	// This should only take 20s from when idle first begins, but allow 30s to
	// avoid any potential race conditions.
	// Once the dialog text appears, click the dialog's "No" button and wait for
	// the dialog to disappear. This is detected by waiting for the static text to
	// disappear.
	chromeVoxText := nodewith.NameStartingWith("Do you want to activate ChromeVox").Role(role.StaticText).Onscreen()
	noButton := nodewith.Name("No, continue without ChromeVox").Role(role.Button).Onscreen()
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("wait for and interact with the ChromeVox hint dialog",
		ui.WithTimeout(30*time.Second).WaitUntilExists(chromeVoxText),
		ui.LeftClickUntil(noButton, ui.Gone(chromeVoxText)),
	)(ctx); err != nil {
		s.Fatal("Failed to show and interact with the ChromeVox hint dialog: ", err)
	}

	// Use the speech monitor to ensure that the spoken announcement was given.
	err = sm.Consume(ctx, []a11y.SpeechExpectation{a11y.NewRegexExpectation("Do you want to activate ChromeVox, the built-in screen reader for Chrome OS*")})
	if err != nil {
		s.Fatal("Failed to verify the ChromeVox hint announcement: ", err)
	}
}
