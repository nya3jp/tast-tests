// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type glideTypingTestParam struct {
	floatLayout bool
	inputMethod ime.InputMethod
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardGlideTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(3 * time.Minute),
		Params: []testing.Param{
			{
				Name:      "tablet_docked",
				Fixture:   fixture.TabletVK,
				ExtraAttr: []string{"group:input-tools-upstream"},
				Val: glideTypingTestParam{
					floatLayout: false,
					inputMethod: ime.EnglishUS,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:      "tablet_floating",
				Fixture:   fixture.TabletVK,
				ExtraAttr: []string{"group:input-tools-upstream"},
				Val: glideTypingTestParam{
					floatLayout: true,
					inputMethod: ime.EnglishUSWithInternationalKeyboard,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard}),
			},
			{
				Name:      "clamshell_a11y_docked",
				Fixture:   fixture.ClamshellVK,
				ExtraAttr: []string{"group:input-tools-upstream"},
				Val: glideTypingTestParam{
					floatLayout: false,
					inputMethod: ime.EnglishUS,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:      "clamshell_a11y_floating",
				Fixture:   fixture.ClamshellVK,
				ExtraAttr: []string{"group:input-tools-upstream"},
				Val: glideTypingTestParam{
					floatLayout: true,
					inputMethod: ime.EnglishUS,
				},
				ExtraSearchFlags: util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:    "tablet_docked_lacros",
				Fixture: fixture.LacrosTabletVK,
				Val: glideTypingTestParam{
					floatLayout: false,
					inputMethod: ime.EnglishUS,
				},
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:    "tablet_floating_lacros",
				Fixture: fixture.LacrosTabletVK,
				Val: glideTypingTestParam{
					floatLayout: true,
					inputMethod: ime.EnglishUSWithInternationalKeyboard,
				},
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUSWithInternationalKeyboard}),
			},
			{
				Name:    "clamshell_a11y_docked_lacros",
				Fixture: fixture.LacrosClamshellVK,
				Val: glideTypingTestParam{
					floatLayout: false,
					inputMethod: ime.EnglishUS,
				},
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
			{
				Name:    "clamshell_a11y_floating_lacros",
				Fixture: fixture.LacrosClamshellVK,
				Val: glideTypingTestParam{
					floatLayout: true,
					inputMethod: ime.EnglishUS,
				},
				ExtraAttr:         []string{"group:input-tools-upstream"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
			},
		},
	})
}

func VirtualKeyboardGlideTyping(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	inputMethod := s.Param().(glideTypingTestParam).inputMethod
	shouldFloatLayout := s.Param().(glideTypingTestParam).floatLayout

	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatalf("Faield to install and activate input method %q: %v", inputMethod, err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	// Launch inputs test web server.
	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	// Select the input field being tested.
	vkbCtx := vkb.NewContext(cr, tconn)

	glideTypingWord, ok := data.TypingMessageHello.GetInputData(inputMethod)
	if !ok {
		s.Fatalf("Test Data for input method %v does not exist", inputMethod)
	}
	keySeq := glideTypingWord.CharacterKeySeq

	if shouldFloatLayout {
		if err := uiauto.Combine("set VK to floating mode",
			vkbCtx.ShowVirtualKeyboard(),
			vkbCtx.SetFloatingMode(uc, true),
			vkbCtx.HideVirtualKeyboard(),
		)(ctx); err != nil {
			s.Fatal("Failed to set VK to floating mode: ", err)
		}
		defer func(ctx context.Context) {
			if err := uiauto.Combine("reset VK to docked mode",
				vkbCtx.ShowVirtualKeyboard(),
				vkbCtx.SetFloatingMode(uc, false),
				vkbCtx.HideVirtualKeyboard(),
			)(ctx); err != nil {
				s.Log("Failed to reset VK to docked mode: ", err)
			}
		}(cleanupCtx)

	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Define glide typing user action including validating the result.
	glideTypingUserAction := func(testScenario string, inputField testserver.InputField, isGlideTypingEnabled bool) uiauto.Action {
		// Wait for the glide typing engine to be ready.
		// The wait is required for the betty boards.
		testing.Sleep(ctx, 2*time.Second)
		// Define result validation function.
		// Should submit the last key if glide typing disabled.
		validateResultFunc := its.ValidateResult(inputField, keySeq[len(keySeq)-1])
		if isGlideTypingEnabled {
			// Check if input field text exactly matches glide typing result.
			// Select candidate from suggestion bar is also acceptable.
			validateResultFunc = func(ctx context.Context) error {
				if err := util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), glideTypingWord.ExpectedText)(ctx); err != nil {
					s.Log("Input field text does not match glide typing gesture: ", err)
					s.Log("Check if it is in suggestion bar")
					return uiauto.Combine("selecting candidate from suggestion bar",
						vkbCtx.SelectFromSuggestionIgnoringCase(glideTypingWord.ExpectedText),
						its.ValidateResult(inputField, glideTypingWord.ExpectedText),
					)(ctx)
				}
				return nil
			}
		}

		return uiauto.UserAction("VK glide typing",
			vkbCtx.GlideTyping(keySeq, validateResultFunc),
			uc,
			&useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeInputField:   string(inputField),
					useractions.AttributeTestScenario: testScenario,
					useractions.AttributeFeature:      useractions.FeatureGlideTyping,
				},
			},
		)
	}

	validateGlideTyping := func(testScenario string, inputField testserver.InputField, isGlideTypingEnabled bool) uiauto.Action {
		return uiauto.Combine("validate glide typing",
			vkbCtx.HideVirtualKeyboard(),
			its.Clear(inputField),
			its.ClickFieldUntilVKShown(inputField),
			glideTypingUserAction(testScenario, inputField, isGlideTypingEnabled),
		)
	}

	util.RunSubTest(ctx, s, cr, "default", validateGlideTyping("Glide typing is enabled by default", testserver.TextAreaInputField, true))
	util.RunSubTest(ctx, s, cr, "not_applicable", validateGlideTyping("Glide typing should not work on non-appliable fields", testserver.PasswordInputField, false))

	if err := imesettings.SetGlideTyping(uc, inputMethod, false)(ctx); err != nil {
		s.Fatal("Failed to disable glide typing: ", err)
	}

	util.RunSubTest(ctx, s, cr, "disable", validateGlideTyping("Glide typing can be disabled in IME setting", testserver.TextAreaInputField, false))

	if err := imesettings.SetGlideTyping(uc, inputMethod, true)(ctx); err != nil {
		s.Fatal("Failed to disable glide typing: ", err)
	}

	util.RunSubTest(ctx, s, cr, "re-enable", validateGlideTyping("Glide typing can be enabled in IME setting", testserver.TextAreaInputField, true))
}
