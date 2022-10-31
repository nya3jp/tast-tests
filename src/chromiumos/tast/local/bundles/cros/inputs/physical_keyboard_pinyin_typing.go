// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
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
		Func:         PhysicalKeyboardPinyinTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Pinyin physical keyboard works",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:             "simplified",
				Fixture:          fixture.ClamshellNonVK,
				Val:              ime.ChinesePinyin,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.ChinesePinyin}),
			},
			{
				Name:             "traditional",
				Fixture:          fixture.ClamshellNonVK,
				Val:              ime.ChineseTraditionalPinyin,
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.ChineseTraditionalPinyin}),
			},
			{
				Name:              "simplified_lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				Val:               ime.ChinesePinyin,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.ChinesePinyin}),
			},
			{
				Name:              "traditional_lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				Val:               ime.ChineseTraditionalPinyin,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.ChineseTraditionalPinyin}),
			},
		},
	})
}

func PhysicalKeyboardPinyinTyping(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

	ui := uiauto.New(tconn)

	subtests := []struct {
		name     string
		scenario string
		action   uiauto.Action
	}{
		{
			// Type something and check that the text is split into syllables.
			name:     "TypePinyinShowsSyllables",
			scenario: "verify text is split into syllables",
			action:   its.ValidateInputOnField(inputField, kb.TypeAction("nihao"), "ni hao"),
		},
		{
			// Type something and press space to submit the top candidate.
			name:     "SpaceSubmitsTopCandidate",
			scenario: "Use SPACE to submit the top candidate",
			action: uiauto.Combine("type and press space",
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
		{
			// Type something and use arrow keys to select a different candidate.
			// Press space to submit the candidate, which might only be a prefix.
			name:     "ArrowKeyAndSpaceSubmitsPartialCandidate",
			scenario: "Use arrow keys to select a different candidate and submit using SPACE key",
			action: uiauto.Combine("type and press space",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihao"),
				kb.AccelAction("Down"),
				kb.AccelAction("Down"),
				kb.AccelAction("Up"),
				util.GetNthCandidateTextAndThen(tconn, 1, func(prefix string) uiauto.Action {
					return uiauto.Combine("press space and verify text",
						kb.AccelAction("Space"),
						util.WaitForFieldTextToSatisfy(tconn, inputField.Finder(), fmt.Sprintf("starts with %s", prefix), func(text string) bool {
							// TODO(b/190248867): Check the suffix as well.
							return strings.HasPrefix(text, prefix)
						}),
					)
				}),
			),
		},
		{
			// Type something and press number key to submit a candidate, which might only be a prefix.
			name:     "NumberKeySubmitsCandidate",
			scenario: "Use number key to submit candidate",
			action: uiauto.Combine("bring up candidates and select with number key",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("nihao"),
				util.GetNthCandidateTextAndThen(tconn, 3, func(prefix string) uiauto.Action {
					return uiauto.Combine("press number and verify text",
						// The number labels on the candidates are one-based.
						kb.TypeAction("4"),
						util.WaitForFieldTextToSatisfy(tconn, inputField.Finder(), fmt.Sprintf("starts with %s", prefix), func(text string) bool {
							// TODO(b/190248867): Check the suffix as well.
							return strings.HasPrefix(text, prefix)
						}),
					)
				}),
			),
		},
		{
			// Press shift to switch to Raw input mode.
			name:     "ShiftTogglesLanguageMode",
			scenario: "Press SHIFT to switch to Raw input mode",
			action: uiauto.Combine("bring up candidates and select with number key",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.AccelAction("Shift"),
				kb.TypeAction("ni "),
				kb.AccelAction("Shift"),
				kb.TypeAction("hao"),
				util.GetNthCandidateTextAndThen(tconn, 0, func(text string) uiauto.Action {
					return uiauto.Combine("press space and verify text",
						kb.AccelAction("Space"),
						ui.WaitUntilGone(util.PKCandidatesFinder),
						util.WaitForFieldTextToBe(tconn, inputField.Finder(), "ni "+text),
					)
				}),
			),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.name))

			if err := uiauto.UserAction(
				"Chinese Pinyin PK input",
				subtest.action,
				uc, &useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.scenario,
						useractions.AttributeInputField:   string(inputField),
						useractions.AttributeFeature:      useractions.FeaturePKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}
}
