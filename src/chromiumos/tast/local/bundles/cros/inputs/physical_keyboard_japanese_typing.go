// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardJapaneseTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Japanese physical keyboard works",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:             "us",
				Fixture:          fixture.ClamshellNonVK,
				Val:              ime.JapaneseWithUSKeyboard,
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.JapaneseWithUSKeyboard}),
			},
			{
				Name:             "jp",
				Fixture:          fixture.ClamshellNonVK,
				Val:              ime.Japanese,
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.Japanese}),
			},
			{
				Name:              "us_lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				Val:               ime.JapaneseWithUSKeyboard,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.JapaneseWithUSKeyboard}),
			},
			{
				Name:              "jp_lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				Val:               ime.Japanese,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.Japanese}),
			},
		},
	})
}

// validateInputFieldFromNthCandidate returns an action that gets the candidate at the specified position and checks if the input field has the same value.
func validateInputFieldFromNthCandidate(its *testserver.InputsTestServer, tconn *chrome.TestConn, inputField testserver.InputField, n int) uiauto.Action {
	return util.GetNthCandidateTextAndThen(tconn, n, func(text string) uiauto.Action {
		return util.WaitForFieldTextToBe(tconn, inputField.Finder(), text)
	})
}

func PhysicalKeyboardJapaneseTyping(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	im := s.Param().(ime.InputMethod)

	s.Log("Set current input method to: ", im)
	if err := im.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatalf("Failed to set input method to %v: %v: ", im, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, im.Name)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	inputField := testserver.TextAreaInputField

	// Focus on the input field and wait for a small duration.
	// This is needed as the Japanese IME has a bug where typing immediately after
	// a new focus will leave the first character unconverted.
	// TODO(b/191213378): Remove this once the bug is fixed.
	if err := its.ClickFieldAndWaitForActive(inputField)(ctx); err != nil {
		s.Fatal("Failed to wait for input field to activate: ", err)
	}
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	ui := uiauto.New(tconn)

	subtests := []struct {
		name     string
		scenario string
		action   uiauto.Action
	}{
		// Type and check that the text field has the correct Hiragana.
		{
			name:     "TypeRomajiShowsHiragana",
			scenario: "Type Romaji and check correct Hiragana",
			action:   its.ValidateInputOnField(inputField, kb.TypeAction("nihongo"), "にほんご"),
		},
		// Type and press Tab/Shift+Tab to select different candidates.
		// The text field should show the selected candidate.
		{
			name:     "TabCyclesThroughCandidates",
			scenario: "Type and press Tab/Shift+Tab to select candidate",
			action: uiauto.Combine("Use Tab key to Cycle through candidates",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihongo"),
				uiauto.Repeat(3, kb.AccelAction("Tab")),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 2),
				kb.AccelAction("Shift+Tab"),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 1),
			),
		},
		// Type and press arrow keys to select different candidates.
		// The text field should show the selected candidate.
		{
			name:     "ArrowKeysCycleThroughCandidates",
			scenario: "Use arrow keys to cycle through candidates",
			action: uiauto.Combine("cycle through candidates with arrow keys",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihongo"),
				uiauto.Repeat(3, kb.AccelAction("Down")),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 2),
				kb.AccelAction("Up"),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 1),
			),
		},
		// Type and press Tab/Arrow keys to go through multiple pages of candidates.
		// The text field should show the selected candidate.
		{
			name:     "TabAndArrowKeysCyclesThroughPages",
			scenario: "Use Tab/Arrow keys to flip pages",
			action: uiauto.Combine("cycle through pages with tab and arrow keys",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihongo"),
				// The Japanese IME shows a max of 9 candidates per page.
				uiauto.Repeat(10, kb.AccelAction("Tab")),
				uiauto.Repeat(5, kb.AccelAction("Down")),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 5),
			),
		},
		// Type and press a number key to select the candidate with that number.
		// The text field should show the selected candidate.
		{
			name:     "NumberKeySelectsCandidate",
			scenario: "Use number key to select the candidate",
			action: uiauto.Combine("bring up candidates and select with number key",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihongo"),
				kb.AccelAction("Tab"),
				uiauto.Repeat(5, kb.AccelAction("Tab")),
				kb.TypeAction("3"),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 2),
			),
		},
		// Type and press space, which should select the first conversion candidate and hide the candidates window.
		{
			name:     "SpaceSelectsTopConversionCandidate",
			scenario: "Use SPACE key to select the first conversion candidate",
			action: uiauto.Combine("bring up the conversion candidates window",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihongo"),
				// Pop up the conversion candidates window to find the top conversion candidate.
				kb.AccelAction("Space"),
				kb.AccelAction("Space"),
				util.GetNthCandidateTextAndThen(tconn, 0, func(text string) uiauto.Action {
					return uiauto.Combine("retype and press space to select default candidate",
						its.ClearThenClickFieldAndWaitForActive(inputField),
						kb.TypeAction("nihongo"),
						kb.AccelAction("Space"),
						ui.WaitUntilGone(util.PKCandidatesFinder),
						util.WaitForFieldTextToBe(tconn, inputField.Finder(), text),
					)
				}),
			),
		},
		// Type and press space multiple times to go through different conversion candidates.
		// The text field should show the selected candidate.
		{
			name:     "SpaceCyclesThroughConversionCandidates",
			scenario: "Type and press SPACE multiple times to go through different conversion candidates",
			action: uiauto.Combine("type some text",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihongo"),
				uiauto.Repeat(5, kb.AccelAction("Space")),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 4),
			),
		},
		// Type and Tab several times to select a candidate.
		// Press Enter, which should submit the selected candidate and hide the candidates window.
		{
			name:     "EnterSubmitsCandidate",
			scenario: "Type and Tab several times to select a candidate",
			action: uiauto.Combine("type some text",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihongo"),
				uiauto.Repeat(3, kb.AccelAction("Tab")),
				util.GetNthCandidateTextAndThen(tconn, 2, func(text string) uiauto.Action {
					return uiauto.Combine("press enter and verify text",
						kb.AccelAction("Enter"),
						ui.WaitUntilGone(util.PKCandidatesFinder),
						util.WaitForFieldTextToBe(tconn, inputField.Finder(), text),
					)
				}),
			),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.name))

			if err := uiauto.UserAction(
				"Japanese PK input",
				subtest.action,
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.scenario,
						useractions.AttributeFeature:      useractions.FeaturePKTyping,
						useractions.AttributeInputField:   string(inputField),
					},
				},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}
}
