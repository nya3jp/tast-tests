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
	"chromiumos/tast/local/bundles/cros/inputs/autocorrect"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAutocorrect,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that virtual keyboard can perform typing with autocorrects",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Params: []testing.Param{
			{
				Name:    "en_us_tablet",
				Fixture: fixture.TabletVK,
				Val: autocorrect.TestCase{
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "helol",
					CorrectWord:  "hello",
					UndoMethod:   autocorrect.ViaPopupUsingMouse,
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:    "en_us_a11y",
				Fixture: fixture.ClamshellVK,
				Val: autocorrect.TestCase{
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "helol",
					CorrectWord:  "hello",
					UndoMethod:   autocorrect.ViaPopupUsingMouse,
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:    "es_es_tablet",
				Fixture: fixture.TabletVK,
				Val: autocorrect.TestCase{
					InputMethod:  ime.SpanishSpain,
					MisspeltWord: "espanol",
					CorrectWord:  "español",
					UndoMethod:   autocorrect.NotApplicable,
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.SpanishSpain}),
			},
			{
				Name:    "es_es_a11y",
				Fixture: fixture.ClamshellVK,
				Val: autocorrect.TestCase{
					InputMethod:  ime.SpanishSpain,
					MisspeltWord: "espanol",
					CorrectWord:  "español",
					UndoMethod:   autocorrect.NotApplicable,
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.SpanishSpain}),
			},
			{
				Name:    "fr_fr_tablet",
				Fixture: fixture.TabletVK,
				Val: autocorrect.TestCase{
					InputMethod:  ime.FrenchFrance,
					MisspeltWord: "francais",
					CorrectWord:  "français",
					UndoMethod:   autocorrect.NotApplicable,
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:    "fr_fr_a11y",
				Fixture: fixture.ClamshellVK,
				Val: autocorrect.TestCase{
					InputMethod:  ime.FrenchFrance,
					MisspeltWord: "francais",
					CorrectWord:  "français",
					UndoMethod:   autocorrect.NotApplicable,
				},
				ExtraAttr:        []string{"group:input-tools-upstream"},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:              "en_us_tablet_lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "helol",
					CorrectWord:  "hello",
					UndoMethod:   autocorrect.ViaPopupUsingMouse,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:              "en_us_a11y_lacros",
				Fixture:           fixture.LacrosClamshellVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.EnglishUS,
					MisspeltWord: "helol",
					CorrectWord:  "hello",
					UndoMethod:   autocorrect.ViaPopupUsingMouse,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:              "es_es_tablet_lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.SpanishSpain,
					MisspeltWord: "espanol",
					CorrectWord:  "español",
					UndoMethod:   autocorrect.NotApplicable,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.SpanishSpain}),
			},
			{
				Name:              "es_es_a11y_lacros",
				Fixture:           fixture.LacrosClamshellVK,
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.SpanishSpain,
					MisspeltWord: "espanol",
					CorrectWord:  "español",
					UndoMethod:   autocorrect.NotApplicable,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.SpanishSpain}),
			},
			{
				Name:              "fr_fr_tablet_lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.FrenchFrance,
					MisspeltWord: "francais",
					CorrectWord:  "français",
					UndoMethod:   autocorrect.NotApplicable,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
			{
				Name:              "fr_fr_a11y_lacros",
				Fixture:           fixture.LacrosClamshellVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Val: autocorrect.TestCase{
					InputMethod:  ime.FrenchFrance,
					MisspeltWord: "francais",
					CorrectWord:  "français",
					UndoMethod:   autocorrect.NotApplicable,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
			},
		},
	})
}

func VirtualKeyboardAutocorrect(ctx context.Context, s *testing.State) {
	testCase := s.Param().(autocorrect.TestCase)
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	inputMethod := testCase.InputMethod
	s.Logf("Set current input method to: %q", inputMethod)
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatalf("Failed to install and set input method to %q: %v: ", inputMethod, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	if err := imesettings.SetVKAutoCapitalization(uc, inputMethod, false)(ctx); err != nil {
		s.Fatal("Failed to disable auto-capitalization in IME settings: ", err)
	}

	defer func() {
		if err := inputMethod.ResetSettings(tconn)(cleanupCtx); err != nil {
			// Only log errors in cleanup.
			s.Log("Failed to reset IME settings: ", err)
		}
	}()

	const inputField = testserver.TextAreaInputField
	if err := uiauto.Combine("validate VK autocorrect",
		its.Clear(inputField),
		its.ClickFieldUntilVKShown(inputField),
	)(ctx); err != nil {
		s.Fatal("Failed to clear then click input field to show VK: ", err)
	}

	triggerACAction := uiauto.Combine("validate VK autocorrect",
		vkbCtx.TapKeys(strings.Split(testCase.MisspeltWord, "")),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.MisspeltWord),
		vkbCtx.TapKey("space"),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.CorrectWord+" "),
	)

	if err := uiauto.UserAction("VK autocorrect",
		triggerACAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: fmt.Sprintf("Auto correct %q to %q", testCase.MisspeltWord, testCase.CorrectWord),
				useractions.AttributeFeature:      useractions.FeatureAutoCorrection,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate VK autocorrect: ", err)
	}

	switch testCase.UndoMethod {
	case autocorrect.ViaBackspace:
		s.Fatal("ViaBackspace undo method disappears after unknown timeout so testing not automatable")

	case autocorrect.ViaPopupUsingPK:
		s.Fatal("ViaPopupUsingPK undo method is not applicable for VK")

	case autocorrect.ViaPopupUsingMouse:
		// AssistAutoCorrect flag's features. Only available for US-English.
		if !testCase.InputMethod.Equal(ime.EnglishUS) {
			s.Fatalf("ViaPopupUsingMouse undo method is not applicable for: %q", testCase.InputMethod)
		}

		ui := uiauto.New(tconn)
		textFieldFinder := nodewith.Name("textAreaInputField").Role(role.TextField)
		textContentFinder := nodewith.Role(role.StaticText).Ancestor(textFieldFinder)

		if err := ui.WaitUntilExists(textContentFinder)(ctx); err != nil {
			s.Fatal("Cannot find text content: ", err)
		}

		boundingBox, err := ui.Location(ctx, textContentFinder)
		if err != nil {
			s.Fatal("Cannot find text content location coords: ", err)
		}

		// Need to click on the word, but text field has an extra space at the end,
		// hence centre point shifted slightly leftwards.
		clickTarget := coords.NewPoint(
			boundingBox.Left+(boundingBox.Width/3),
			boundingBox.Top+(boundingBox.Height/2))

		undoWindowFinder := nodewith.ClassName("UndoWindow").Role(role.Window)
		undoButtonFinder := nodewith.Name("Undo").Role(role.Button).Ancestor(undoWindowFinder)

		validateUndoAction := uiauto.Combine("validate VK autocorrect undo via popup using mouse",
			mouse.Click(tconn, clickTarget, mouse.LeftButton),
			ui.WaitUntilExists(undoButtonFinder),
			ui.LeftClick(undoButtonFinder),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), testCase.MisspeltWord+" "),
		)

		if err := uiauto.UserAction("Undo VK auto-correction",
			validateUndoAction,
			uc,
			&useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeTestScenario: "Undo auto-correction via mouse",
					useractions.AttributeFeature:      useractions.FeatureAutoCorrection,
				},
			},
		)(ctx); err != nil {
			s.Fatal("Failed to validate VK autocorrect undo via popup using mouse: ", err)
		}
	}
}
