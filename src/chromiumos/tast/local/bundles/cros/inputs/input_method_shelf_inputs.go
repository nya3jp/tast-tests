// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input/voice"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var testMessages = []data.Message{
	data.VoiceMessageHello,
	data.HandwritingMessageHello,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputMethodShelfInputs,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test input functions triggered from IME tray",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		Data:         data.ExtractExternalFiles(testMessages, []ime.InputMethod{ime.DefaultInputMethod}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Pre:               pre.NonVKClamshellReset,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "informational",
				Pre:               pre.NonVKClamshellReset,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "fixture",
				Fixture:           fixture.ClamshellNonVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
		},
	})
}

func InputMethodShelfInputs(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome
	var tconn *chrome.TestConn
	var uc *useractions.UserContext
	if strings.Contains(s.TestName(), "fixture") {
		cr = s.FixtValue().(fixture.FixtData).Chrome
		tconn = s.FixtValue().(fixture.FixtData).TestAPIConn
		uc = s.FixtValue().(fixture.FixtData).UserContext
		uc.SetTestName(s.TestName())
	} else {
		cr = s.PreValue().(pre.PreData).Chrome
		tconn = s.PreValue().(pre.PreData).TestAPIConn
		uc = s.PreValue().(pre.PreData).UserContext
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Setup CRAS Aloop for audio test.
	cleanup, err := voice.EnableAloop(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to load Aloop: ", err)
	}
	defer cleanup(ctx)

	if err := imesettings.EnableInputOptionsInShelf(uc, true)(ctx); err != nil {
		s.Fatal("Failed to show input options in shelf: ", err)
	}

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	ui := uiauto.New(tconn)
	inputField := testserver.TextAreaInputField
	testIME := ime.DefaultInputMethod

	imeMenuTrayButtonFinder := nodewith.Name("IME menu button").Role(role.Button)
	voiceInputItem := nodewith.Name("Voice").HasClass("SystemMenuButton")
	voicePrivacyConfirmButton := nodewith.Name("Got it").HasClass("voice-got-it")
	handwritingInputItem := nodewith.Name("Handwriting").HasClass("SystemMenuButton")
	handwritingPrivacyConfirmButton := nodewith.Name("Got it").HasClass("button")
	emojiInputMenuItem := nodewith.Name("Emojis").HasClass("SystemMenuButton")

	voiceInputData, ok := data.VoiceMessageHello.GetInputData(testIME)
	if !ok {
		s.Fatal("Failed to get voice test data of input method: ", testIME)
	}
	hwInputData, ok := data.HandwritingMessageHello.GetInputData(testIME)
	if !ok {
		s.Fatal("Failed to get handwriting test data of input method: ", testIME)
	}

	voiceInputUserAction := func() uiauto.Action {
		scenario := "Voice input triggered from IME tray"

		verifyAudioInputAction := uiauto.Combine(scenario,
			its.Clear(inputField),
			uiauto.Sleep(time.Second),
			its.ClickFieldAndWaitForActive(inputField),
			ui.LeftClick(imeMenuTrayButtonFinder),
			ui.LeftClick(voiceInputItem),
			ui.LeftClick(voicePrivacyConfirmButton),
			uiauto.Sleep(time.Second),
			func(ctx context.Context) error {
				return voice.AudioFromFile(ctx, s.DataPath(voiceInputData.VoiceFile))
			},
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), voiceInputData.ExpectedText),
		)

		return uiauto.UserAction("Voice input",
			verifyAudioInputAction,
			uc,
			&useractions.UserActionCfg{
				Callback: func(ctx context.Context, actionErr error) error {
					vkbCtx := vkb.NewContext(cr, tconn)
					return vkbCtx.HideVirtualKeyboard()(ctx)
				},
				Attributes: map[string]string{
					useractions.AttributeInputField:   string(inputField),
					useractions.AttributeTestScenario: scenario,
					useractions.AttributeFeature:      useractions.FeatureVoiceInput,
				},
				Tags: []useractions.ActionTag{useractions.ActionTagIMEShelf},
			},
		)
	}

	handwritingInputUserAction := func() uiauto.Action {
		scenario := "Verify handwriting input triggered from IME tray"

		hwFilePath := s.DataPath(hwInputData.HandwritingFile)
		verifyHandWritingInputAction := uiauto.Combine(scenario,
			its.Clear(inputField),
			uiauto.Sleep(time.Second),
			its.ClickFieldAndWaitForActive(inputField),
			ui.LeftClick(imeMenuTrayButtonFinder),
			ui.LeftClick(handwritingInputItem),
			// The privacy dialog does not appear on all devices.
			uiauto.IfSuccessThen(
				ui.WithTimeout(2*time.Second).WaitUntilExists(handwritingPrivacyConfirmButton),
				ui.LeftClick(handwritingPrivacyConfirmButton),
			),
			func(ctx context.Context) error {
				hwCtx, err := vkb.NewContext(cr, tconn).NewHandwritingContext(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to initiate handwriting context")
				}
				return uiauto.Combine(scenario,
					its.WaitForHandwritingEngineReadyOnField(hwCtx, inputField, hwFilePath),
					hwCtx.DrawStrokesFromFile(hwFilePath),
					util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), hwInputData.ExpectedText),
				)(ctx)
			},
		)

		return uiauto.UserAction("Handwriting",
			verifyHandWritingInputAction,
			uc,
			&useractions.UserActionCfg{
				Callback: func(ctx context.Context, actionErr error) error {
					vkbCtx := vkb.NewContext(cr, tconn)
					return vkbCtx.HideVirtualKeyboard()(ctx)
				},
				Attributes: map[string]string{
					useractions.AttributeInputField:   string(inputField),
					useractions.AttributeTestScenario: scenario,
					useractions.AttributeFeature:      useractions.FeatureHandWriting,
				},
				Tags: []useractions.ActionTag{useractions.ActionTagIMEShelf},
			},
		)
	}

	emojiInputUserAction := func() uiauto.Action {
		scenario := "Verify emoji input triggered from IME tray"

		inputEmoji := "😄"
		emojiPickerFinder := nodewith.Name("Emoji Picker").Role(role.RootWebArea)
		emojiItem := nodewith.Name(inputEmoji).Ancestor(emojiPickerFinder).First()

		verifyEmojiInputAction := uiauto.Combine(scenario,
			its.Clear(inputField),
			uiauto.Sleep(time.Second),
			its.ClickFieldAndWaitForActive(inputField),
			ui.LeftClick(imeMenuTrayButtonFinder),
			ui.LeftClick(emojiInputMenuItem),
			ui.LeftClick(emojiItem),
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputEmoji),
		)

		return uiauto.UserAction(
			"Input Emoji with Emoji Picker",
			verifyEmojiInputAction,
			uc,
			&useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeInputField:   string(inputField),
					useractions.AttributeTestScenario: scenario,
					useractions.AttributeFeature:      useractions.FeatureEmojiPicker,
				},
				Tags: []useractions.ActionTag{useractions.ActionTagIMEShelf},
			})
	}

	subTests := []struct {
		name   string
		action uiauto.Action
	}{
		{"voice", voiceInputUserAction()},
		{"handwriting", handwritingInputUserAction()},
		{"emoji", emojiInputUserAction()},
	}

	for _, subtest := range subTests {
		util.RunSubTest(ctx, s, cr, subtest.name, subtest.action)
	}
}
