// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		Timeout:      10 * time.Minute,
		Fixture:      "projectorLogin",
		VarDeps: []string{
			"projector.sharedScreencastLink",
		},
	})
}

func SharedScreencast(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn := s.FixtValue().(*projector.FixtData).TestConn
	cr := s.FixtValue().(*projector.FixtData).Chrome

	sharedScreencast := s.RequiredVar("projector.sharedScreencastLink")

	defer faillog.DumpUITreeOnError(ctxForCleanUp, s.OutDir(), s.HasError, tconn)

	// Set up browser.
	// TODO(b/229633861): Also test URL handling in Lacros browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(ctxForCleanUp)

	// Open a new window.
	conn, err := br.NewConn(ctx, sharedScreencast, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to navigate to Projector landing page: ", err)
	}
	defer conn.Close()

	if err := br.ReloadActiveTab(ctx); err != nil {
		s.Fatal("Failed to launch Projector app: ", err)
	}

	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)

	screencastTitle := nodewith.Name("Screencast for Tast (Do not modify)").Role(role.StaticText)
	spinner := nodewith.ClassName("spinner-container").Role(role.GenericContainer)
	shareButton := nodewith.Name("Share").Role(role.Button)
	copyLinkButton := nodewith.Name("Copy link").Role(role.Button)
	translationDropdown := nodewith.Name("English").Role(role.Button)
	french := nodewith.Name("français").Role(role.ListBoxOption)
	searchToolbar := nodewith.Name("Find in transcript").Role(role.Button)
	searchBox := nodewith.Name("Find in transcript").Role(role.TextField)
	searchResult := nodewith.Name("1/1").Role(role.StaticText).Ancestor(nodewith.ClassName("search-result-label"))
	selectedTranscript := nodewith.Name("marks allemands").Role(role.StaticText).Ancestor(nodewith.ClassName("selected"))
	timeElapsed := nodewith.Name("01:47").Role(role.StaticText).Ancestor(nodewith.Name("Time elapsed"))
	timeRemaining := nodewith.Name("01:23").Role(role.StaticText).Ancestor(nodewith.Name("Time remaining"))
	highlightedTranscript := nodewith.Name("01:47").Role(role.StaticText).Ancestor(nodewith.ClassName("transcript highlighted"))
	skipBack := nodewith.Name("Skip back").Role(role.Button)
	skipBackTimeElapsed := nodewith.Name("01:37").Role(role.StaticText).Ancestor(nodewith.Name("Time elapsed"))
	skipBackTimeRemaining := nodewith.Name("01:33").Role(role.StaticText).Ancestor(nodewith.Name("Time remaining"))
	skipBackHighlightedTranscript := nodewith.Name("01:31").Role(role.StaticText).Ancestor(nodewith.ClassName("transcript highlighted"))
	skipAhead := nodewith.Name("Skip ahead").Role(role.Button)
	playButton := nodewith.Name("Play").Role(role.Button)
	pauseButton := nodewith.Name("Pause").Role(role.Button)

	refreshApp := projector.RefreshApp(ctx, tconn)

	// Verify the shared screencast title rendered correctly.
	if err := ui.WithInterval(5*time.Second).RetryUntil(refreshApp, ui.Exists(screencastTitle))(ctx); err != nil {
		s.Fatal("Failed to render shared screencast: ", err)
	}

	if err := projector.DismissOnboardingDialog(ctx, tconn); err != nil {
		s.Fatal("Failed to close the onboarding dialog: ", err)
	}

	if err := uiauto.Combine("copying share link and translating to French",
		// Copy the share link to clipboard.
		ui.WithInterval(5*time.Second).RetryUntil(refreshApp, ui.Exists(shareButton)),
		ui.LeftClickUntil(shareButton, ui.Exists(copyLinkButton)),
		ui.LeftClickUntil(copyLinkButton, ui.Gone(copyLinkButton)),
		// Translate the transcript to French.
		ui.WithInterval(5*time.Second).RetryUntil(refreshApp, ui.Exists(translationDropdown)),
		ui.WaitUntilGone(spinner),
		ui.WithInterval(time.Second).LeftClickUntil(translationDropdown, ui.Exists(french)),
		ui.MakeVisible(french),
		ui.LeftClick(french),
		ui.WaitUntilGone(french),
		// Open the search toolbar.
		ui.LeftClick(searchToolbar),
		ui.WaitUntilExists(searchBox.Focused()),
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
		ui.WithInterval(time.Second).LeftClickUntil(selectedTranscript, ui.Exists(timeElapsed)),
		ui.WaitUntilExists(timeRemaining),
		ui.WaitUntilExists(highlightedTranscript),
		ui.LeftClick(skipBack),
		ui.WaitUntilExists(skipBackTimeElapsed),
		ui.WaitUntilExists(skipBackTimeRemaining),
		// After skipping back 10 seconds, the highlighted
		// transcript should be at the 01:31 timestamp.
		ui.WaitUntilExists(skipBackHighlightedTranscript),
		ui.LeftClick(skipAhead),
		ui.WaitUntilExists(timeElapsed),
		ui.WaitUntilExists(timeRemaining),
		ui.WaitUntilExists(highlightedTranscript),
		ui.WithInterval(time.Second).LeftClickUntil(playButton, ui.Exists(pauseButton)),
		ui.WithInterval(time.Second).LeftClickUntil(pauseButton, ui.Exists(playButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to navigate transcript and media controls: ", err)
	}
}
