// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardMultiwordSuggestion,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks on device multiword suggestions with physical keyboard typing",
		Contacts:     []string{"curtismcmullan@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		HardwareDeps: hwdep.D(hwdep.Model(pre.MultiwordEnabledModels...)),
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture: fixture.ClamshellNonVKWithMultiwordSuggest,
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVKWithMultiwordSuggest,
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func PhysicalKeyboardMultiwordSuggestion(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// PK multiword suggestion only works in English(US).
	inputMethod := ime.EnglishUS

	// Activate function checks the current IME. It does nothing if the given input method is already in-use.
	// It is called here just in case IME has been changed in last test.
	if err := inputMethod.Activate(tconn)(ctx); err != nil {
		s.Fatal("Failed to set IME: ", err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	inputField := testserver.TextAreaInputField
	suggestionWindowFinder := nodewith.HasClass("SuggestionWindowView").Role(role.Window)
	ui := uiauto.New(tconn)

	// TODO(b/224628222): Expecting an ML candidate to remain the same in
	//   tests can be somewhat flakey in future runs of the test. Update
	//   these tests to capture the candidate shown on screen and validate
	//   against that candidate.
	subtests := []struct {
		name     string
		scenario string
		errStr   string
		action   uiauto.Action
	}{
		{
			// Trigger suggestion "how are you" and insert into
			// textfield with tab.
			name:     "AcceptSuggestionWithTab",
			scenario: "verify suggestion appears and accepted with tab key",
			errStr:   "Failed to accept suggestion: %v",
			action: uiauto.Combine("accept multiword suggestion with tab",
				keyboard.TypeAction("ho"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "ho"),
				ui.WaitUntilExists(suggestionWindowFinder),
				keyboard.AccelAction("Tab"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "how are you"),
			),
		},
		{
			// Trigger suggestion "how are you" and insert into
			// textfield with down + enter key.
			name:     "AcceptSuggestionWithDownAndEnter",
			scenario: "verify suggestion appears and accepted with down and enter key",
			errStr:   "Failed to accept suggestion: %v",
			action: uiauto.Combine("accept multiword suggestion with down and enter",
				keyboard.TypeAction("ho"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "ho"),
				ui.WaitUntilExists(suggestionWindowFinder),
				keyboard.AccelAction("Down"),
				keyboard.AccelAction("Enter"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "how are you"),
			),
		},
		{
			// Trigger suggestion "my name is" and dismiss with
			// multiple white space at the end of the text.
			name:     "SuggestionShouldAppearOnlyAtEndOfText",
			scenario: "verify suggestion dismissed with multiple whitespace",
			errStr:   "Failed to dismiss suggestion with whitespace: %v",
			action: uiauto.Combine("dismiss multiword suggestion with multiple whitespace",
				keyboard.TypeAction("my name"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "my name"),
				ui.WaitUntilExists(suggestionWindowFinder),
				keyboard.TypeAction("  "),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "my name  "),
				ui.WaitUntilGone(suggestionWindowFinder),
			),
		},
		{
			// Trigger suggestion "how are you", partially type
			// suggestion, and dismiss suggestion by deleting text
			// beyond suggestion trigger point.
			name:     "SuggestionTrackedAndDismissedWithTextUpdates",
			scenario: "track typing in suggestion and dismiss when deleting past trigger point",
			errStr:   "Failed to dismiss suggestion: %v",
			action: uiauto.Combine("dismiss multiword suggestion by deleting past trigger point",
				keyboard.TypeAction("hi there h"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "hi there h"),
				ui.WaitUntilExists(suggestionWindowFinder),
				keyboard.TypeAction("ow a"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "hi there how a"),
				ui.WaitUntilExists(suggestionWindowFinder),
				keyboard.AccelAction("Backspace"), // "hi there how "
				keyboard.AccelAction("Backspace"), // "hi there how"
				keyboard.AccelAction("Backspace"), // "hi there ho"
				keyboard.AccelAction("Backspace"), // "hi there h"
				ui.WaitUntilExists(suggestionWindowFinder),
				keyboard.AccelAction("Backspace"), // "hi there "
				ui.WaitUntilGone(suggestionWindowFinder),
			),
		},
		{
			// Suggestions should handle newlines gracefully.
			name:     "SuggestionHandlesNewline",
			scenario: "suggestions handles newline gracefully",
			errStr:   "Failed to accept suggestion: %v",
			action: uiauto.Combine("suggestion handles newline gracefully",
				keyboard.TypeAction("goo"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "goo"),
				ui.WaitUntilExists(suggestionWindowFinder),
				keyboard.AccelAction("Enter"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "goo\n"),
				ui.WaitUntilGone(suggestionWindowFinder),
				keyboard.AccelAction("Backspace"),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "goo"),
				ui.WaitUntilExists(suggestionWindowFinder),
				keyboard.AccelAction("Tab"),
				ui.WaitUntilGone(suggestionWindowFinder),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "good morning"),
			),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.name))

			if err := uiauto.UserAction(
				"Multiword suggestion",
				uiauto.Combine("...",
					its.Clear(inputField),
					its.ClickFieldAndWaitForActive(inputField),
					subtest.action,
				),
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.scenario,
						useractions.AttributeInputField:   string(inputField),
						useractions.AttributeFeature:      useractions.FeatureMultiwordSuggestion,
					},
				},
			)(ctx); err != nil {
				s.Fatalf(subtest.errStr, err)
			}
		})
	}
}
