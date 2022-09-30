// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/familylink"
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
		Func:         TranscriptTranslation,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests transcript translation for various user types",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		VarDeps: []string{
			"projector.sharedScreencastLink",
		},
		Params: []testing.Param{
			{
				Name:    "regular_consumer",
				Fixture: "projectorLogin",
			},
			{
				Name:    "supervised_child",
				Fixture: "projectorUnicornLogin",
			},
			{
				Name:    "managed_edu",
				Fixture: "projectorEduLogin",
			},
		},
	})
}

func TranscriptTranslation(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	sharedScreencast := s.RequiredVar("projector.sharedScreencastLink")

	defer faillog.DumpUITreeOnError(ctxForCleanUp, s.OutDir(), s.HasError, tconn)

	// SWA installation is not guaranteed during startup.
	// Wait for installation finished before starting test.
	s.Log("Wait for Screencast app to be installed")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Projector.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	// Tast test projector.SharedScreencast already tests opening
	// share links in Lacros, so a Lacros variant is not needed
	// here.
	if err := projector.OpenSharedScreencast(ctx, tconn, cr, browser.TypeAsh, sharedScreencast); err != nil {
		s.Fatal("Failed to open shared screencast: ", err)
	}

	translationDropdown := nodewith.Name("Language").ClassName("translation-lang")
	french := nodewith.Name("français").Role(role.ListBoxOption)
	frenchText := "marks allemands"
	searchToolbar := nodewith.Name("Find in transcript").Role(role.Button)
	searchBox := nodewith.Name("Find in transcript").Role(role.TextField)
	searchResult := nodewith.Name("1/1").Role(role.StaticText).Ancestor(nodewith.ClassName("search-result-label"))
	selectedTranscript := nodewith.Name(frenchText).Role(role.StaticText).Ancestor(nodewith.ClassName("selected"))

	ui := uiauto.New(tconn).WithTimeout(time.Minute)

	if err := uiauto.Combine("translating transcript to French",
		ui.WaitUntilExists(translationDropdown),
		ui.LeftClickUntil(translationDropdown, ui.Exists(french)),
		ui.MakeVisible(french),
		ui.LeftClickUntil(french, ui.Gone(french)),
		ui.LeftClickUntil(searchToolbar, ui.Exists(searchBox)),
	)(ctx); err != nil {
		s.Fatal("Failed to translate transcript to French: ", err)
	}

	// Typing search term into search box.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	if err := kb.Type(ctx, frenchText); err != nil {
		s.Fatal("Failed to type search term: ", err)
	}

	if err := uiauto.Combine("verifying translation",
		// There should only be one search result in the
		// transcript.
		ui.WaitUntilExists(searchResult),
		// We're searching for "German marks" in French so we
		// know translation worked.
		ui.WaitUntilExists(selectedTranscript),
	)(ctx); err != nil {
		s.Fatal("Failed to verify translation: ", err)
	}
}
