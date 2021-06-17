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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardJapaneseTyping,
		Desc:         "Checks that Japanese physical keyboard works",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
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
	return func(ctx context.Context) error {
		expectedValue, err := util.GetNthCandidateText(ctx, tconn, n)
		if err != nil {
			return err
		}

		if err := its.WaitForFieldValueToBe(inputField, expectedValue)(ctx); err != nil {
			return err
		}

		return nil
	}
}

func PhysicalKeyboardJapaneseTyping(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

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

	clearAndFocus := uiauto.Combine("clear input field and focus",
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
	)

	subtests := []struct {
		Name   string
		Action uiauto.Action
	}{
		{
			Name:   "TypeRomajiShowsHiragana",
			Action: its.ValidateInputOnField(inputField, kb.TypeAction("nihongo"), "にほんご"),
		},
		{
			Name: "TabCyclesThroughCandidates",
			Action: uiauto.Combine("cycle through candidates with tab",
				clearAndFocus,
				kb.TypeAction("nihongo"),
				uiauto.Repeat(3, kb.AccelAction("Tab")),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 2),
				kb.AccelAction("Shift+Tab"),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 1),
			),
		},
		{
			Name: "ArrowKeysCycleThroughCandidates",
			Action: uiauto.Combine("cycle through candidates with arrow keys",
				clearAndFocus,
				kb.TypeAction("nihongo"),
				uiauto.Repeat(3, kb.AccelAction("Down")),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 2),
				kb.AccelAction("Up"),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 1),
			),
		},
		{
			Name: "TabAndArrowKeysCyclesThroughPages",
			Action: uiauto.Combine("cycle through pages with tab and arrow keys",
				clearAndFocus,
				kb.TypeAction("nihongo"),
				// The Japanese IME shows a max of 9 candidates per page.
				uiauto.Repeat(10, kb.AccelAction("Tab")),
				uiauto.Repeat(5, kb.AccelAction("Down")),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 5),
			),
		},
		{
			Name: "NumberKeySelectsCandidate",
			Action: uiauto.Combine("bring up candidates and select with number key",
				clearAndFocus,
				kb.TypeAction("nihongo"),
				kb.AccelAction("Tab"),
				uiauto.Repeat(5, kb.AccelAction("Tab")),
				kb.TypeAction("3"),
				validateInputFieldFromNthCandidate(its, tconn, inputField, 2),
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
