// Copyright 2022 The ChromiumOS Authors.
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
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.DefaultInputMethod}),
		Timeout:      2 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:           fixture.ClamshellNonVK,
				ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
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
				ExtraSoftwareDeps: []string{"lacros"},
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
		inputMethod  ime.InputMethod
		typeAction   string
		expectedText string
	}{
		// TODO(b/237498932): Add other keyboards listed in bug report.
		{
			inputMethod:  ime.EnglishUSWithInternationalKeyboard,
			typeAction:   "abcdefghijklmnopqrstuvwxyz01234!",
			expectedText: "áb©ðéfghíjœøµñóöä®ßþúvåxüæ’¡²³¤¹",
		},
		{
			inputMethod:  ime.Swedish,
			typeAction:   "abcdefghijklmnopqrstuvwxyz01234!",
			expectedText: "ª”©ð€đŋħ→łµnœþ@®ßþ↓“ł»←«}¡@£$¹",
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

	// Pressing the shift key by itself does not have any affect if no caps lock.
	// However if the test failed while caps lock is still enabled, shift will
	// disable it. This is just to make sure the state of the caps lock is clean
	// for the next test that uses the fixture.
	defer keyboard.AccelAction("Altgr")(cleanupCtx)

	for _, testcase := range testCases {
		name := "PKAltgrModifierWorksFor" + testcase.inputMethod.ShortLabel
		scenario := "PK Altgr Modifier Works For " + testcase.inputMethod.Name

		s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(name))
			im := testcase.inputMethod

			s.Log("Set current input method to: ", im)
			if err := im.InstallAndActivateUserAction(uc)(ctx); err != nil {
				s.Fatalf("Failed to set input method to %v: %v: ", im, err)
			}

			if err := uiauto.UserAction(scenario,
				uiauto.Combine(scenario,
					its.Clear(inputField),
					its.ClickFieldAndWaitForActive(inputField),
					keyboard.AccelPressAction("Altgr"),
					keyboard.TypeAction(testcase.typeAction),
					util.WaitForFieldTextToBe(tconn, inputField.Finder(), testcase.expectedText),
					keyboard.AccelReleaseAction("Altgr"),
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
