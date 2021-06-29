// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardInputFields,
		Desc:         "Checks that virtual keyboard works on different input fields",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "us_en_stable",
				Pre:               pre.VKEnabledTablet,
				Val:               ime.INPUTMETHOD_XKB_US_ENG,
				ExtraAttr:         []string{"group:mainline", "informational", "group:input-tools-upstream"},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			}, {
				Name:              "us_en_unstable",
				Pre:               pre.VKEnabledTablet,
				Val:               ime.INPUTMETHOD_XKB_US_ENG,
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			}, {
				Name:              "jp_us_stable",
				Pre:               pre.VKEnabledTablet,
				Val:               ime.INPUTMETHOD_NACL_MOZC_US,
				ExtraAttr:         []string{"group:mainline", "group:input-tools-upstream", "informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			}, {
				Name:              "jp_us_unstable",
				Pre:               pre.VKEnabledTablet,
				Val:               ime.INPUTMETHOD_NACL_MOZC_US,
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			}, {
				Name:              "zh_pinyin_stable",
				Pre:               pre.VKEnabledTablet,
				Val:               ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			}, {
				Name:              "zh_pinyin_unstable",
				Pre:               pre.VKEnabledTablet,
				Val:               ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
		},
	})
}

func VirtualKeyboardInputFields(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	vkbCtx := vkb.NewContext(cr, tconn)

	imeCode := ime.IMEPrefix + string(s.Param().(ime.InputMethodCode))
	s.Logf("Set current input method to: %s", imeCode)
	if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
	}

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	type testData struct {
		inputField      testserver.InputField
		keySeq          []string
		inputSuggestion bool
		expectedText    string
	}

	var subTests []testData

	switch s.Param().(ime.InputMethodCode) {
	case ime.INPUTMETHOD_XKB_US_ENG:
		subTests = []testData{
			{
				inputField:   testserver.TextAreaInputField,
				keySeq:       strings.Split("hello", ""),
				expectedText: "hello",
			}, {
				inputField:   testserver.TextInputField,
				keySeq:       strings.Split("hello", ""),
				expectedText: "hello",
			}, {
				inputField:   testserver.SearchInputField,
				keySeq:       strings.Split("hello", ""),
				expectedText: "hello",
			}, {
				inputField:   testserver.PasswordInputField,
				keySeq:       strings.Split("hello", ""),
				expectedText: "hello",
			}, {
				inputField:   testserver.NumberInputField,
				keySeq:       strings.Split("-123.456", ""),
				expectedText: "-123.456",
			}, {
				inputField:   testserver.EmailInputField,
				keySeq:       []string{"t", "e", "s", "t", "@", "g", "m", "a", "i", "l", ".com"},
				expectedText: "test@gmail.com",
			},
			{
				inputField:   testserver.URLInputField,
				keySeq:       []string{"g", "o", "o", "g", "l", "e", ".com", "/"},
				expectedText: "google.com/",
			},
			{
				inputField:   testserver.TelInputField,
				keySeq:       []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "2", "3"},
				expectedText: "-+,.(),;N123",
			}, {
				inputField:   testserver.TextInputNumericField,
				keySeq:       strings.Split("123456789#0-+", ""),
				expectedText: "123456789#0-+",
			},
		}
		break
	case ime.INPUTMETHOD_NACL_MOZC_US:
		subTests = []testData{
			{
				inputField:   testserver.TextAreaInputField,
				keySeq:       strings.Split("konnnitiha", ""),
				expectedText: "こんにちは",
			}, {
				inputField:   testserver.TextInputField,
				keySeq:       strings.Split("konnnitiha", ""),
				expectedText: "こんにちは",
			}, {
				inputField:   testserver.SearchInputField,
				keySeq:       strings.Split("konnnitiha", ""),
				expectedText: "こんにちは",
			}, {
				inputField:   testserver.PasswordInputField,
				keySeq:       strings.Split("konnnitiha", ""),
				expectedText: "konnnitiha",
			}, {
				inputField:   testserver.NumberInputField,
				keySeq:       strings.Split("-123.456", ""),
				expectedText: "-123.456",
			}, {
				inputField:   testserver.EmailInputField,
				keySeq:       strings.Split("konnnitiha", ""),
				expectedText: "こんにちは",
			}, {
				inputField:   testserver.URLInputField,
				keySeq:       strings.Split("konnnitiha", ""),
				expectedText: "こんにちは",
			}, {
				inputField:   testserver.TelInputField,
				keySeq:       []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "0"},
				expectedText: "-+,.(),;N10",
			}, {
				inputField:   testserver.TextInputNumericField,
				keySeq:       strings.Split("123456789#0-+", ""),
				expectedText: "123456789#0-+",
			},
		}
		break
	case ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED:
		subTests = []testData{
			{
				inputField:      testserver.TextAreaInputField,
				keySeq:          strings.Split("nihao", ""),
				inputSuggestion: true,
				expectedText:    "你好",
			}, {
				inputField:      testserver.TextInputField,
				keySeq:          strings.Split("nihao", ""),
				inputSuggestion: true,
				expectedText:    "你好",
			}, {
				inputField:      testserver.SearchInputField,
				keySeq:          strings.Split("nihao", ""),
				inputSuggestion: true,
				expectedText:    "你好",
			}, {
				inputField:   testserver.PasswordInputField,
				keySeq:       strings.Split("nihao", ""),
				expectedText: "nihao",
			}, {
				inputField:   testserver.NumberInputField,
				keySeq:       strings.Split("-123.456", ""),
				expectedText: "-123.456",
			}, {
				inputField:      testserver.EmailInputField,
				keySeq:          strings.Split("nihao", ""),
				inputSuggestion: true,
				expectedText:    "你好",
			}, {
				inputField:      testserver.URLInputField,
				keySeq:          strings.Split("nihao", ""),
				inputSuggestion: true,
				expectedText:    "你好",
			}, {
				inputField:   testserver.TelInputField,
				keySeq:       []string{"-", "+", ",", ".", "(", ")", "Pause", "Wait", "N", "1", "0"},
				expectedText: "-+,.(),;N10",
			}, {
				inputField:   testserver.TextInputNumericField,
				keySeq:       strings.Split("123456789#0-+", ""),
				expectedText: "123456789#0-+",
			},
		}
		break
	default:
		s.Fatalf("%s is not supported", imeCode)
	}

	for _, subtest := range subTests {
		s.Run(ctx, string(subtest.inputField), func(ctx context.Context, s *testing.State) {
			defer func() {
				outDir := filepath.Join(s.OutDir(), string(subtest.inputField))
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+string(subtest.inputField))

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}()

			inputField := subtest.inputField

			if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
				s.Fatal("Failed to click input field to show virtual keyboard: ", err)
			}

			if err := vkbCtx.TapKeysIgnoringCase(subtest.keySeq)(ctx); err != nil {
				s.Fatalf("Failed to tap keys %v: %v", subtest.keySeq, err)
			}

			// some IMEs need to manually select from candidates to submit.
			if subtest.inputSuggestion {
				if err := vkbCtx.SelectFromSuggestion(subtest.expectedText)(ctx); err != nil {
					s.Fatalf("Failed to select %s from suggestions: %v", subtest.expectedText, err)
				}
			}

			// Password input is a special case. The value is presented with placeholder "•".
			// Using PasswordTextField field to verify the outcome.
			if inputField == testserver.PasswordInputField {
				if err := util.WaitForFieldTextToBe(tconn, inputField.Finder(), strings.Repeat("•", len(subtest.keySeq)))(ctx); err != nil {
					s.Fatal("Failed to verify input: ", err)
				}

				if err := util.WaitForFieldTextToBe(tconn, testserver.PasswordTextField.Finder(), subtest.expectedText)(ctx); err != nil {
					s.Fatal("Failed to verify password input: ", err)
				}
			} else {
				if err := util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), subtest.expectedText)(ctx); err != nil {
					s.Fatal("Failed to verify input: ", err)
				}
			}
		})
	}
}
