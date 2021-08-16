// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEmojiSuggestion,
		Desc:         "Checks emoji suggestions with physical keyboard typing",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
			Pre:               pre.NonVKClamshell,
		}, {
			Name:              "guest",
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
			Pre:               pre.NonVKClamshellInGuest,
		}, {
			Name:              "incognito",
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
			Pre:               pre.NonVKClamshell,
		}, {
			// Only run informational tests in consumer mode.
			Name:              "informational",
			Pre:               pre.NonVKClamshell,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}}})
}

func PhysicalKeyboardEmojiSuggestion(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// PK emoji suggestion only works in English(US).
	inputMethod := ime.EnglishUS

	// Activate function checks the current IME. It does nothing if the given input method is already in-use.
	// It is called here just in case IME has been changed in last test.
	if err := inputMethod.Activate(tconn)(ctx); err != nil {
		s.Fatal("Failed to set IME: ", err)
	}
	its, err := testserver.LaunchInMode(ctx, cr, tconn, strings.HasSuffix(s.TestName(), "incognito"))
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	const inputField = testserver.TextInputField

	const (
		word  = "yum"
		emoji = "🤤"
	)

	ui := uiauto.New(tconn)

	emojiCandidateWindowFinder := nodewith.HasClass("SuggestionWindowView").Role(role.Window)
	emojiCharFinder := nodewith.Name(emoji).Ancestor(emojiCandidateWindowFinder).HasClass("StyledLabel")

	validateEmojiSuggestion := func(shouldSuggest bool) uiauto.Action {
		return uiauto.Combine("validate emoji suggestion",
			its.Clear(inputField),
			its.ClickFieldAndWaitForActive(inputField),
			kb.TypeAction(word),
			kb.AccelAction("SPACE"),
			func(ctx context.Context) error {
				if shouldSuggest {
					// Select emoji and wait for the candidate window disappear.
					return uiauto.Combine("select emoji suggestion",
						ui.LeftClick(emojiCharFinder),
						ui.WaitUntilGone(emojiCandidateWindowFinder),
						util.WaitForFieldTextToBe(tconn, inputField.Finder(), word+" "+emoji),
					)(ctx)
				}
				// Otherwise check emoji suggestion window does not appear in 1s.
				// Sleep is necessary here, otherwise it immediately returns success because of UI reflection delay.
				testing.Sleep(ctx, time.Second)
				return uiauto.Combine("continue to input without emoji suggestion",
					ui.WaitUntilGone(emojiCandidateWindowFinder),
					kb.TypeAction(word),
					util.WaitForFieldTextToBe(tconn, inputField.Finder(), word+" "+word),
				)(ctx)
			},
		)
	}

	testName := "suggestion"
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

		if err := validateEmojiSuggestion(true)(ctx); err != nil {
			s.Fatal("Failed to validate enabled emoji suggestion: ", err)
		}
	})

	// Test disabling emoji suggestion.
	// Launch setting from emoji suggestion window and toggle it off.
	testName = "disable"
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

		if err := its.Clear(inputField)(ctx); err != nil {
			s.Fatalf("Failed to clear input field %s: %v", string(inputField), err)
		}

		learnMoreFinder := nodewith.Name("Learn more").Ancestor(emojiCandidateWindowFinder).HasClass("ImageButton")

		if err := uiauto.Combine("launch emoji suggestion setting",
			its.ClickFieldAndWaitForActive(inputField),
			// Use the first data to test "Learn more".
			kb.TypeAction(word),
			kb.AccelAction("SPACE"),
			ui.LeftClick(learnMoreFinder),
			ui.WaitUntilGone(emojiCandidateWindowFinder),
		)(ctx); err != nil {
			s.Fatal("Failed to validate emoji suggestion: ", err)
		}

		// User should be on suggestion setting page in OS settings.
		imeSettings := imesettings.New(tconn)

		if err := uiauto.Combine("toggle off emoji suggestion and validate no emoji suggestion",
			imeSettings.WaitUntilEmojiSuggestion(cr, tconn, true),
			imeSettings.ToggleEmojiSuggestions(tconn),
			imeSettings.WaitUntilEmojiSuggestion(cr, tconn, false),
			imeSettings.Close(),
			validateEmojiSuggestion(false),
		)(ctx); err != nil {
			s.Fatal("Failed to toggle off emoji suggestion and validate no emoji suggestion: ", err)
		}
	})

	// Test re-enabling emoji suggestion.
	// Launch setting from OS Settings and toggle it on.
	testName = "re-enable"
	s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

		imeSettings, err := imesettings.LaunchAtSuggestionSettingsPage(ctx, tconn, cr)
		if err != nil {
			s.Fatal("Failed to launch OS settings: ", err)
		}
		if err := uiauto.Combine("toggle on emoji suggestion and validate emoji suggestion",
			imeSettings.ToggleEmojiSuggestions(tconn),
			imeSettings.WaitUntilEmojiSuggestion(cr, tconn, true),
			imeSettings.Close(),
			validateEmojiSuggestion(true),
		)(ctx); err != nil {
			s.Fatal("Failed to toggle on emoji suggestion and validate emoji suggestion: ", err)
		}
	})
}
