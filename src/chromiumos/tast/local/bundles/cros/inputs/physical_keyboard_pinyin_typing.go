// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardPinyinTyping,
		Desc:         "Checks that Pinyin physical keyboard works",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
	})
}

func PhysicalKeyboardPinyinTyping(ctx context.Context, s *testing.State) {
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
	imeCode := ime.IMEPrefix + string(ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED)

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

	ui := uiauto.New(tconn)

	subtests := []struct {
		Name   string
		Action uiauto.Action
	}{
		// Type something and check that the text is split into syllables.
		{
			Name:   "TypePinyinShowsSyllables",
			Action: its.ValidateInputOnField(inputField, kb.TypeAction("nihao"), "ni hao"),
		},
		// Type something and press space to submit the top candidate.
		{
			Name: "SpaceSubmitsTopCandidate",
			Action: uiauto.Combine("type and press space",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihao"),
				util.GetNthCandidateTextAndThen(tconn, 0, func(text string) uiauto.Action {
					return uiauto.Combine("press space and verify text",
						kb.AccelAction("Space"),
						ui.WaitUntilGone(util.PKCandidatesFinder),
						util.WaitForFieldTextToBe(tconn, inputField.Finder(), text),
					)
				}),
			),
		},
		// Type something and use arrow keys to select a different candidate.
		// Press space to submit the candidate, which might only be a prefix.
		{
			Name: "ArrowKeyAndSpaceSubmitsPartialCandidate",
			Action: uiauto.Combine("type and press space",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihao"),
				kb.AccelAction("Down"),
				kb.AccelAction("Down"),
				kb.AccelAction("Up"),
				util.GetNthCandidateTextAndThen(tconn, 1, func(prefix string) uiauto.Action {
					return uiauto.Combine("press space and verify text",
						kb.AccelAction("Space"),
						util.WaitForFieldTextToSatisfy(tconn, inputField.Finder(), fmt.Sprintf("starts with %s", prefix), func(text string) bool {
							return strings.HasPrefix(text, prefix)
						}),
					)
				}),
			),
		},
		// Type something and press number key to submit a candidate, which might only be a prefix.
		{
			Name: "NumberKeySubmitsCandidate",
			Action: uiauto.Combine("bring up candidates and select with number key",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihao"),
				util.GetNthCandidateTextAndThen(tconn, 3, func(prefix string) uiauto.Action {
					return uiauto.Combine("press number and verify text",
						kb.TypeAction("4"),
						util.WaitForFieldTextToSatisfy(tconn, inputField.Finder(), fmt.Sprintf("starts with %s", prefix), func(text string) bool {
							return strings.HasPrefix(text, prefix)
						}),
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
