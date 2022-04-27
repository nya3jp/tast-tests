// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SharedScreencast,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Opens a shared screencast in viewer mode",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "projectorLoginExtendedFeaturesDisabled",
		VarDeps: []string{
			"projector.sharedScreencast",
		},
	})
}

func SharedScreencast(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*projector.FixtData).TestConn
	cr := s.FixtValue().(*projector.FixtData).Chrome

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	closeOnboardingButton := nodewith.Name("Skip tour").Role(role.Button)
	shareButton := nodewith.Name("Share").Role(role.StaticText)
	copyLinkButton := nodewith.Name("Copy link").Role(role.Button)
	translationDropdown := nodewith.Name("Language").ClassName("translation-lang")
	french := nodewith.Name("français").Role(role.ListBoxOption)
	searchToolbar := nodewith.Name("Find in transcript").Role(role.Button)
	searchBox := nodewith.Name("Find in transcript").Role(role.TextField)
	searchResult := nodewith.Name("1/1").Role(role.StaticText).Ancestor(nodewith.ClassName("search-result-label"))
	selectedTranscript := nodewith.Name("marks allemands").Role(role.StaticText).Ancestor(nodewith.ClassName("selected"))
	timeElapsed := nodewith.Name("01:47").Role(role.StaticText).Ancestor(nodewith.Name("Time elapsed"))
	timeRemaing := nodewith.Name("01:23").Role(role.StaticText).Ancestor(nodewith.Name("Time remaining"))
	highlightedTranscript := nodewith.Name("01:47").Role(role.StaticText).Ancestor(nodewith.ClassName("transcript highlighted"))
	skipBack := nodewith.Name("Skip back").Role(role.Button)
	skipBackTimeElapsed := nodewith.Name("01:37").Role(role.StaticText).Ancestor(nodewith.Name("Time elapsed"))
	skipBackTimeRemaing := nodewith.Name("01:33").Role(role.StaticText).Ancestor(nodewith.Name("Time remaining"))
	skipBackHighlightedTranscript := nodewith.Name("01:31").Role(role.StaticText).Ancestor(nodewith.ClassName("transcript highlighted"))
	skipAhead := nodewith.Name("Skip ahead").Role(role.Button)
	playButton := nodewith.Name("Play").Role(role.Button)
	pauseButton := nodewith.Name("Pause").Role(role.Button)

	sharedScreencast := s.RequiredVar("projector.sharedScreencast")

	// Set up browser.
	// TODO(b/229633861): Also test URL handling in Lacros browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(ctx)

	// Open a new window.
	conn, err := br.NewConn(ctx, sharedScreencast, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to navigate to Projector landing page: ", err)
	}
	defer conn.Close()

	if err := br.ReloadActiveTab(ctx); err != nil {
		s.Fatal("Failed to launch Projector app: ", err)
	}

	// Dismiss the onboarding dialog, if it exists. Since each
	// user only sees the onboarding flow a maximum of three
	// times, the onboarding dialog may not appear.
	if err := ui.WaitUntilExists(closeOnboardingButton)(ctx); err == nil {
		s.Log("Dismissing the onboarding dialog")
		if err = ui.LeftClickUntil(closeOnboardingButton, ui.Gone(closeOnboardingButton))(ctx); err != nil {
			s.Fatal("Failed to close the onboarding dialog: ", err)
		}
	}

	if err := uiauto.Combine("copying share link and translating to French",
		// Copy the share link to clipboard.
		ui.WaitUntilExists(shareButton),
		ui.LeftClickUntil(shareButton, ui.Exists(copyLinkButton)),
		ui.LeftClickUntil(copyLinkButton, ui.Gone(copyLinkButton)),
		// Translate the transcript to French.
		ui.LeftClickUntil(translationDropdown, ui.Exists(french)),
		ui.MakeVisible(french),
		ui.LeftClickUntil(french, ui.Gone(french)),
		ui.LeftClickUntil(searchToolbar, ui.Exists(searchBox)),
	)(ctx); err != nil {
		s.Fatal("Failed to copy share link and translate to French: ", err)
	}

	// Check the shareable link copied to clipboard.
	data, err := ash.ClipboardTextData(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to retrieve clipboard data: ", err)
	}
	if data != sharedScreencast {
		s.Fatalf("Clipboard data doesn't match share link: expected %s actual %s", sharedScreencast, data)
	}

	// Typing search term into search box.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	if err := kb.Type(ctx, "marks allemands"); err != nil {
		s.Fatal("Failed to type search term: ", err)
	}

	if err := uiauto.Combine("navigating transcript and media controls",
		// There should only be one search result in the
		// transcript.
		ui.WaitUntilExists(searchResult),
		// We're searching for "German marks" in French so we
		// know translation worked.
		ui.WaitUntilExists(selectedTranscript),
		ui.LeftClickUntil(selectedTranscript, ui.Exists(timeElapsed)),
		ui.WaitUntilExists(timeRemaing),
		ui.WaitUntilExists(highlightedTranscript),
		ui.WithInterval(time.Second).LeftClickUntil(skipBack, ui.Exists(skipBackTimeElapsed)),
		ui.WaitUntilExists(skipBackTimeRemaing),
		// After skipping back 10 seconds, the highlighted
		// transcript should be at the 01:31 timestamp.
		ui.WaitUntilExists(skipBackHighlightedTranscript),
		ui.WithInterval(time.Second).LeftClickUntil(skipAhead, ui.Exists(timeElapsed)),
		ui.WaitUntilExists(timeRemaing),
		ui.WaitUntilExists(highlightedTranscript),
		ui.LeftClickUntil(playButton, ui.Exists(pauseButton)),
		ui.LeftClickUntil(pauseButton, ui.Exists(playButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to navigate transcript and media controls: ", err)
	}
}
