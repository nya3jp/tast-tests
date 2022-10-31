// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardZhuyinTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Zhuyin physical keyboard works",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.ChineseZhuyin}),
		Params: []testing.Param{
			{
				Fixture: fixture.ClamshellNonVK,
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func PhysicalKeyboardZhuyinTyping(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	im := ime.ChineseZhuyin

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
			// Type something and check that the symbols automatically form Chinese characters.
			name:     "TypeZhuyinConvertsToChineseCharacters",
			scenario: "verify Zhuyin symbols automatically convert to chinese characters",
			action:   its.ValidateInputOnField(inputField, kb.TypeAction("z06wu35j/ jp6"), "繁體中文"),
		},
		{
			// Type symbols without tone should show the symbols.
			name:     "TypeZhuyinWithoutToneShowsSymbols",
			scenario: "Type symbols without tone should show symbols",
			action:   its.ValidateInputOnField(inputField, kb.TypeAction("5j/"), "ㄓㄨㄥ"),
		},
		{
			// Type Zhuyin replaces corresponding initial/medial/final.
			name:     "TypeZhuyinReplacesCorrespondingInitialMedialFinal",
			scenario: "Type Zhuyin without tones to replace the corresponding initial/medial/final",
			action:   its.ValidateInputOnField(inputField, kb.TypeAction("5j/125qwertyasdfghzxcvbnujm8ik,9ol.0p;/-"), "ㄙㄩㄦ"),
		},
		{
			// Type various tone keys to convert to Chinese characters.
			name:     "TypeZhuyinTonesConvertsToChinese",
			scenario: "Type various tone keys to convert to Chinese characters",
			// Test each character separately so that the IME doesn't adjust the character based on previous characters.
			// Esc will clear the composition for the next character.
			action: uiauto.Combine("type Zhuyin with tones and verify the character",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("g3"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "使"),
				kb.AccelAction("Esc"),
				kb.TypeAction("su4"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "逆"),
				kb.AccelAction("Esc"),
				kb.TypeAction("5j/ "),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "中"),
				kb.AccelAction("Esc"),
				kb.TypeAction("586"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "紮"),
				kb.AccelAction("Esc"),
				kb.TypeAction("m "),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "瘀"),
				kb.AccelAction("Esc"),
				kb.TypeAction("up3"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "尹"),
				kb.AccelAction("Esc"),
				kb.TypeAction(",4"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "誒"),
			),
		},
		{
			// Type backspace to delete symbols one by one.
			name:     "TypeBackspaceDeletes",
			scenario: "Type backspace to delete symbols one by one",
			action: uiauto.Combine("type some text and press backspace repeatedly",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("5j/ 5j/"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "中ㄓㄨㄥ"),
				kb.AccelAction("Backspace"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "中ㄓㄨ"),
				kb.AccelAction("Backspace"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "中ㄓ"),
				kb.AccelAction("Backspace"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "中"),
				kb.AccelAction("Backspace"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), ""),
			),
		},
		{
			// Press SPACE to show candidates window after tone key.
			name:     "TypeSpaceShowsCandidates",
			scenario: "Press SPACE to show candidates window after tone key",
			action: uiauto.Combine("type SPACE to show candidates window",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("5j/ "),
				kb.AccelAction("Space"),
				ui.WaitUntilExists(util.PKCandidatesFinder.First()),
			),
		},
		{
			// Press arrow keys and down arrow to select alternate candidates.
			name:     "TypeSpaceShowsCandidates",
			scenario: "Press arrow keys and down arrow to select alternate candidates",
			action: uiauto.Combine("type something, press arrow keys, and down arrow to show candidates window",
				its.ClearThenClickFieldAndWaitForActive(inputField),
				kb.TypeAction("z06wu35j/ jp6"),
				kb.AccelAction("Left"),
				kb.AccelAction("Left"),
				kb.AccelAction("Down"),
				util.GetNthCandidateTextAndThen(tconn, 1, func(text string) uiauto.Action {
					return uiauto.Combine("select another candidate and press enter to confirm it",
						kb.AccelAction("Down"),
						kb.AccelAction("Enter"),
						ui.WaitUntilGone(util.PKCandidatesFinder),
						util.WaitForFieldTextToBe(tconn, inputField.Finder(), "繁體鍾文"),
					)
				}),
			),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.name))

			if err := uiauto.UserAction(
				"Zhuyin PK input",
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
