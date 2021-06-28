// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var inputFieldTestIMEs = []ime.InputMethodCode{
	ime.INPUTMETHOD_NACL_MOZC_US,
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
	ime.INPUTMETHOD_XKB_US_ENG,
}

var inputFieldToMessage = map[testserver.InputField]data.Message{
	testserver.TextAreaInputField:    data.TypingMessageHello,
	testserver.TextInputField:        data.TypingMessageHello,
	testserver.SearchInputField:      data.TypingMessageHello,
	testserver.PasswordInputField:    data.TypingMessagePassword,
	testserver.NumberInputField:      data.TypingMessageNumber,
	testserver.EmailInputField:       data.TypingMessageEmail,
	testserver.URLInputField:         data.TypingMessageURL,
	testserver.TelInputField:         data.TypingMessageTel,
	testserver.TextInputNumericField: data.TypingMessageTextNumber,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardInputFields,
		Desc:         "Checks that virtual keyboard works on different input fields",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools-upstream", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabledTablet,
		Timeout:      time.Duration(len(inputFieldTestIMEs)) * time.Duration(len(inputFieldToMessage)) * time.Minute,
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "unstable",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
		},
	})
}

func VirtualKeyboardInputFields(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// TODO(b/177777412): Enable VK typing tests on auto-cap fields.
	// TODO(b/182960492): Enable vk typing test on '*' char on number input field.
	for _, inputMethod := range inputFieldTestIMEs {
		// Setup input method
		imeCode := ime.IMEPrefix + string(inputMethod)
		s.Logf("Set current input method to: %s", imeCode)
		if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
			s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
		}

		for inputField, message := range inputFieldToMessage {
			inputData, ok := message.GetInputData(inputMethod)
			if !ok {
				s.Fatalf("Test Data for input method %v does not exist", inputMethod)
			}
			testName := string(inputMethod) + "-" + string(inputField)
			s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
				if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
					s.Fatal("Failed to show VK: ", err)
				}

				defer func() {
					// Cleanup states.
					if err := vkbCtx.ClearInputField(inputField.Finder())(ctx); err != nil {
						s.Log("Failed to clear input field: ", err)
					}

					outDir := filepath.Join(s.OutDir(), testName)
					faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

					if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
						s.Log("Failed to hide virtual keyboard: ", err)
					}
				}()

				if err := vkbCtx.TapKeysIgnoringCase(inputData.KeySeq)(ctx); err != nil {
					s.Fatalf("Failed to tap keys %v: %v", inputData.KeySeq, err)
				}

				// some IMEs need to manually select from candidates to submit.
				if inputData.SubmitFromSuggestion {
					if err := vkbCtx.SelectFromSuggestion(inputData.ExpectedText)(ctx); err != nil {
						s.Fatalf("Failed to select %s from suggestions: %v", inputData.ExpectedText, err)
					}
				}

				// Password input is a special case. The value is presented with placeholder "•".
				// Using PasswordTextField field to verify the outcome.
				if inputField == testserver.PasswordInputField {
					if err := util.WaitForFieldTextToBe(tconn, inputField.Finder(), strings.Repeat("•", len(inputData.KeySeq)))(ctx); err != nil {
						s.Fatal("Failed to verify input: ", err)
					}

					if err := util.WaitForFieldTextToBe(tconn, testserver.PasswordTextField.Finder(), inputData.ExpectedText)(ctx); err != nil {
						s.Fatal("Failed to verify password input: ", err)
					}
				} else {
					if err := util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputData.ExpectedText)(ctx); err != nil {
						s.Fatal("Failed to verify input: ", err)
					}
				}
			})
		}
	}
}
