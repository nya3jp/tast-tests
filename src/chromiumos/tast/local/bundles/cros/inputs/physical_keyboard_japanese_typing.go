// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardJapaneseTyping,
		Desc:         "Checks that Japanese physical keyboard works",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "us",
				Val:  ime.INPUTMETHOD_NACL_MOZC_US,
			},
			{
				Name: "jp",
				Val:  ime.INPUTMETHOD_NACL_MOZC_JP,
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
	cr := s.PreValue().(pre.PreData).Chrome

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	imeCode := ime.IMEPrefix + string(s.Param().(ime.InputMethodCode))

	s.Logf("Set current input method to: %s", imeCode)
	if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

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
		Name   string
		Action uiauto.Action
	}{
		// Type and check that the text field has the correct Hiragana.
		{
			Name:   "TypeRomajiShowsHiragana",
			Action: its.ValidateInputOnField(inputField, kb.TypeAction("nihongo"), "にほんご"),
		},
		// Type and press Tab/Shift+Tab to select different candidates.
		// The text field should show the selected candidate.
		{
			Name: "TabCyclesThroughCandidates",
			Action: uiauto.Combine("cycle through candidates with tab",
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
			Name: "ArrowKeysCycleThroughCandidates",
			Action: uiauto.Combine("cycle through candidates with arrow keys",
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
			Name: "TabAndArrowKeysCyclesThroughPages",
			Action: uiauto.Combine("cycle through pages with tab and arrow keys",
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
			Name: "NumberKeySelectsCandidate",
			Action: uiauto.Combine("bring up candidates and select with number key",
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
			Name: "SpaceSelectsTopConversionCandidate",
			Action: uiauto.Combine("bring up the conversion candidates window",
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
			Name: "SpaceCyclesThroughConversionCandidates",
			Action: uiauto.Combine("type some text",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihongo"),
				uiauto.Repeat(5, kb.AccelAction("Space")),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 4),
			),
		},
		// Type and Tab several times to select a candidate.
		// Press Enter, which should submit the selected candidate and hide the candidates window.
		{
			Name: "EnterSubmitsCandidate",
			Action: uiauto.Combine("type some text",
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
		s.Run(ctx, subtest.Name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.Name))

			if err := subtest.Action(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}
}
