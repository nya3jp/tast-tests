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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/input/voice"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Dictation,
		Desc: "Tests that the Dictation feature can be used to input text using voice",
		Contacts: []string{
			"akihiroota@chromium.org",      // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr: []string{"group:mainline", "informational"},
		// Load audio file used for Dictation.
		Data:         []string{"voice_en_hello.wav"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func Dictation(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Enable Dictation.
	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Dictation, true); err != nil {
		s.Fatal("Failed to enable Dictation: ", err)
	}

	// Ensure Dictation is off at the end of this test.
	defer func(ctx context.Context) {
		if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Dictation, false); err != nil {
			s.Fatal("Failed to disable Dictation: ", err)
		}
	}(cleanupCtx)

	dialogText := nodewith.NameContaining("Dictation sends your voice to Google").Onscreen()
	continueButton := nodewith.Name("Continue").ClassName("MdTextButton").Onscreen()

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Close Dictation dialog",
		ui.WithTimeout(5*time.Second).WaitUntilExists(dialogText),
		ui.LeftClick(continueButton),
		ui.WithTimeout(5*time.Second).WaitUntilGone(dialogText),
	)(ctx); err != nil {
		s.Fatal("Failed to close the Dictation dialog: ", err)
	}

	// Open a new tab with a text area.
	c, err := a11y.NewTabWithHTML(ctx, cr, "<textarea class='myTextArea'></textarea>")
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	defer c.Close()

	textArea := nodewith.Role(role.TextField).HasClass("myTextArea").Onscreen()
	if err := uiauto.Combine("Focus text field",
		ui.WithTimeout(5*time.Second).WaitUntilExists(textArea),
		ui.FocusAndWait(textArea),
	)(ctx); err != nil {
		s.Fatal("Failed to focus the text area: ", err)
	}

	// Toggle Dictation.
	// Open a connection to the keyboard.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	if err := ew.Accel(ctx, "Search+D"); err != nil {
		s.Fatal("Error when presing Search + D to toggle Dictation on: ", err)
	}

	// Play audio file.
	if err := uiauto.Combine("Play audio file",
		ui.Sleep(time.Second),
		func(ctx context.Context) error {
			return voice.AudioFromFile(ctx, s.DataPath("voice_en_hello.wav"))
		},
	)(ctx); err != nil {
		s.Fatal("Failed to play audio file: ", err)
	}

	// Turn dictation off.
	if err := ew.Accel(ctx, "Search+D"); err != nil {
		s.Fatal("Error when pressing Search + D to toggle Dictation off: ", err)
	}

	// Ensure the spoken text was typed.
	textAreaWithContent := nodewith.Attribute("value", "hello").Role(role.TextField).HasClass("myTextArea").Onscreen()
	if err := uiauto.Combine("Verify text input",
		ui.WithTimeout(5*time.Second).WaitUntilExists(textAreaWithContent),
	)(ctx); err != nil {
		s.Fatal("Failed to verify text input: ", err)
	}
}
