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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromevoxHint,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A test that verifies the behavior of the ChromeVox hint in OOBE. This is a feature that is activated after 20s of idle on the OOBE welcome page. After 20s of idle, we show a dialog and give a spoken announcement with instructions for activating ChromeVox",
		Contacts: []string{
			"akihiroota@chromium.org",      // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{{
			Name: "accept_dialog",
			Val:  true,
		}, {
			Name: "dismiss_dialog",
			Val:  false,
		}},
	})
}

func ChromevoxHint(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// This feature is disabled in dev mode, so pass in the flag to explicitly enable it.
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")), chrome.ExtraArgs("--enable-oobe-chromevox-hint-timer-for-dev-mode"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}

	defer crastestclient.Unmute(cleanupCtx)

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

	chromeVoxText := nodewith.NameStartingWith("Do you want to activate ChromeVox").Role(role.StaticText).Onscreen()
	noButton := nodewith.Name("No, continue without ChromeVox").Role(role.Button).Onscreen()
	yesButton := nodewith.Name("Yes, activate ChromeVox").Role(role.Button).Onscreen()
	var speechExpectations []a11y.SpeechExpectation
	var actions uiauto.Action
	ui := uiauto.New(tconn)
	acceptDialog := s.Param().(bool)

	if acceptDialog {
		// If the dialog is accepted, we should get two speech utterances. The first
		// is the same as below. The second is the ChromeVox welcome message, that
		// indicates that ChromeVox is on.
		speechExpectations = []a11y.SpeechExpectation{
			a11y.NewRegexExpectation("Do you want to activate ChromeVox, the built-in screen reader for Chrome OS*"),
			a11y.NewRegexExpectation("ChromeVox spoken feedback is ready"),
		}
		actions = uiauto.Combine("wait for and interact with the ChromeVox hint dialog",
			ui.WithTimeout(30*time.Second).WaitUntilExists(chromeVoxText),
			ui.LeftClickUntil(yesButton, ui.Gone(chromeVoxText)),
		)
	} else {
		// If the dialog is dismissed, we should only get one speech utterance that
		// asks the user if they want to activate ChromeVox.
		speechExpectations = []a11y.SpeechExpectation{
			a11y.NewRegexExpectation("Do you want to activate ChromeVox, the built-in screen reader for Chrome OS*"),
		}
		actions = uiauto.Combine("wait for and interact with the ChromeVox hint dialog",
			ui.WithTimeout(30*time.Second).WaitUntilExists(chromeVoxText),
			ui.LeftClickUntil(noButton, ui.Gone(chromeVoxText)),
		)
	}

	// Execute actions.
	// Wait for the ChromeVox hint dialog to be shown on-screen.
	// Detect this by waiting for the presence of the dialog's static text,
	// since the dialog itself has no name.
	// This should only take 20s from when idle first begins, but allow 30s to
	// avoid any potential race conditions.
	// Once the dialog text appears, click either the 'Yes' or 'No' button and
	// wait for the dialog to disappear. This is detected by waiting for the
	// static text to disappear.
	if err := actions(ctx); err != nil {
		s.Fatal("Failed to show and interact with the ChromeVox hint dialog: ", err)
	}

	// Lastly, ensure that the correct speech is given by the TTS engine.
	if err := sm.Consume(ctx, speechExpectations); err != nil {
		s.Fatal("Failed to verify the speech utterances for the ChromeVox hint: ", err)
	}
}
