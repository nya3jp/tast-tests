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
		Func:         PhysicalKeyboardAltgr,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that user can lock altgr modifier key on physical keyboard",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard, ime.Swedish}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:           fixture.ClamshellNonVK,
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "informational",
				Fixture:           fixture.ClamshellNonVK,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
		},
	})
}

func PhysicalKeyboardAltgr(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	testCases := []struct {
		inputMethod         ime.InputMethod
		typeAction          string
		expectedText        string
		expectedShiftedText string
	}{
		{
			inputMethod:         ime.EnglishUSWithInternationalKeyboard,
			typeAction:          "abcdefghijklmnopqrstuvwxyz0123456789",
			expectedText:        "áb©ðéfghíjœøµñóöä®ßþúvåxüæ’¡²³¤€¼½¾‘",
			expectedShiftedText: "ÁB¢ÐÉFGHÍJŒØµÑÓÖÄ®§ÞÚVÅXÜÆ£",
		},
		{
			inputMethod:         ime.Swedish,
			typeAction:          "abcdefghijklmnopqrstuvwxyz0123456789",
			expectedText:        "ª”©ð€đŋħ→łµnœþ@®ßþ↓“ł»←«}¡@£$€¥{[]",
			expectedShiftedText: "º’©Ð¢ªŊĦıŁºNŒÞΩ®§Þ↑‘Ł>¥<°¹²³¼¢⅝÷«»",
		},
		{
			inputMethod:         ime.Norwegian,
			typeAction:          "abcdefghijklmnopqrstuvwxyz0123456789",
			expectedText:        "ª”©ð€đŋħ→łµnœπ@®ßþ↓“ł»←«}¡@£$½¥{[]",
			expectedShiftedText: "º’©Ð¢ªŊĦıŁºNŒΠΩ™§Þ↑‘Ł>¥<°¹²³¼‰⅝÷«»",
		},
		{
			inputMethod:         ime.EnglishUK,
			typeAction:          "abcdefghijklmnopqrstuvwxyz0123456789",
			expectedText:        "á”çðéđŋħíłµnóþ@¶ßŧú“ẃ»ý«}¹€½[]",
			expectedShiftedText: "Á’ÇÐÉªŊĦÍŁºNÓÞΩ®§ŦÚ‘Ẃ>Ý<°¡½⅓¼⅜⅝⅞™±",
		},
		{
			inputMethod:         ime.Polish,
			typeAction:          "abcdefghijklmnopqrstuvwxyz0123456789",
			expectedText:        "ą”ćðęæŋ’→ə…łµńóþπ©śß↓„œź←ż»≠²³¢€½§·«",
			expectedShiftedText: "Ą“ĆÐĘÆŊ•↔Ə∞ŃÓÞΩ®Ś™↑‘ŒŹ¥Ż°¡¿£¼‰∧≈¾±",
		},
		{
			inputMethod:         ime.DutchNetherlands,
			typeAction:          "abcdefghijklmnopqrstuvwxyz0123456789",
			expectedText:        "áb©ðéfghíjœøµñóöä®ßþúvåxüæ’¡²³¤€¼½¾‘",
			expectedShiftedText: "ÁB¢ÐÉFGHÍJŒØµÑÓÖÄ®§ÞÚVÅXÜÆ£",
		},
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	inputField := testserver.TextAreaInputField

	defer its.CloseAll(cleanupCtx)

	for _, testcase := range testCases {
		name := "PKAltgrModifierWorksFor" + testcase.inputMethod.ShortLabel
		scenario := "Verify PK Altgr Modifier Works For " + testcase.inputMethod.Name

		s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
			// Reset Altgr, in case Altgr is in a held-down state (if release action did not get run due to failures)
			defer keyboard.AccelAction("Altgr")(cleanupCtx)
			defer keyboard.AccelAction("Shift")(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(name))
			im := testcase.inputMethod

			s.Log("Set current input method to: ", im)
			if err := im.InstallAndActivateUserAction(uc)(ctx); err != nil {
				s.Fatalf("Failed to set input method to %v: %v: ", im, err)
			}

			if err := uiauto.UserAction("Verify PK Altgr Modifer Output",
				uiauto.Combine("Verify PK Altgr Modifier Output",
					its.Clear(inputField),
					its.ClickFieldAndWaitForActive(inputField),
					keyboard.AccelPressAction("Altgr"),
					keyboard.TypeAction(testcase.typeAction),
					util.WaitForFieldTextToBe(tconn, inputField.Finder(), testcase.expectedText),
					its.Clear(inputField),
					its.ClickFieldAndWaitForActive(inputField),
					keyboard.AccelPressAction("Shift"),
					keyboard.TypeAction(testcase.typeAction),
					keyboard.AccelReleaseAction("Altgr"),
					keyboard.AccelReleaseAction("Shift"),
					util.WaitForFieldTextToBe(tconn, inputField.Finder(), testcase.expectedShiftedText),
				),
				uc,
				&useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: scenario,
						useractions.AttributeFeature:      useractions.FeaturePKTyping,
						useractions.AttributeInputMethod:  im.Name,
					},
				},
			)(ctx); err != nil {
				s.Fatal("Failed to validate altgr: ", err)
			}
		})
	}
}
