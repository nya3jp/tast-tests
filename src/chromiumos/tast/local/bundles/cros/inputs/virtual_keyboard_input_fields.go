// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardInputFields,
		Desc:         "Checks that virtual keyboard works on different input fields",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:essential-inputs"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabled(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "us_en_stable",
				Val:               ime.INPUTMETHOD_XKB_US_ENG,
				ExtraHardwareDeps: pre.InputsStableModels,
			}, {
				Name:              "us_en_unstable",
				Val:               ime.INPUTMETHOD_XKB_US_ENG,
				ExtraHardwareDeps: pre.InputsUnstableModels,
			},
			{
				Name:              "jp_us_stable",
				Val:               ime.INPUTMETHOD_NACL_MOZC_US,
				ExtraHardwareDeps: pre.InputsStableModels,
			}, {
				Name:              "jp_us_unstable",
				Val:               ime.INPUTMETHOD_NACL_MOZC_US,
				ExtraHardwareDeps: pre.InputsUnstableModels,
			},
			{
				Name:              "zh_pinyin_stable",
				Val:               ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
				ExtraHardwareDeps: pre.InputsStableModels,
			}, {
				Name:              "zh_pinyin_unstable",
				Val:               ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
				ExtraHardwareDeps: pre.InputsUnstableModels,
			},
		},
	})
}

func VirtualKeyboardInputFields(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Get current input method. Change IME for testing and revert it back in teardown.
	imeCode := string(s.Param().(ime.InputMethodCode))
	originalInputMethod, err := vkb.GetCurrentInputMethod(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current input method: ", err)
	} else if originalInputMethod != imeCode {
		cleanupCtx := ctx
		var cancel func()
		ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		defer func(ctx context.Context) {
			s.Logf("Changing back input method to: %s", originalInputMethod)
			if err := vkb.SetCurrentInputMethod(ctx, tconn, originalInputMethod); err != nil {
				s.Logf("Failed to set input method to %s: %v", originalInputMethod, err)
			}
		}(cleanupCtx)

		s.Logf("Set current input method to: %s", imeCode)
		if err := vkb.SetCurrentInputMethod(ctx, tconn, imeCode); err != nil {
			s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
		}

		// To install a new input method, it requires downloading and installing resources, it can take up to 10s.
		// TODO(b/157686038) A better solution to identify decoder status.
		// Decoder works async in returning status to frontend IME and self loading.
		testing.Sleep(ctx, 10*time.Second)
	}

	its, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
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
				expectedText: "•••••",
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
				keySeq:       strings.Split("123456789*#0-+", ""),
				expectedText: "123456789*#0-+",
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
				expectedText: "••••••••••",
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
				keySeq:       strings.Split("123456789*#0-+", ""),
				expectedText: "123456789*#0-+",
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
				expectedText: "•••••",
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
				keySeq:       strings.Split("123456789*#0-+", ""),
				expectedText: "123456789*#0-+",
			},
		}
		break
	default:
		s.Fatalf("%s is not supported", imeCode)
	}

	for _, subtest := range subTests {
		s.Run(ctx, string(subtest.inputField), func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
			inputField := subtest.inputField

			if err := inputField.ClickUntilVKShown(ctx, tconn); err != nil {
				s.Fatal("Failed to click input field to show virtual keyboard: ", err)
			}

			defer func() {
				if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}()

			if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
				s.Fatal("Failed to wait for virtual keyboard shown and locationed: ", err)
			}

			if err := vkb.TapKeys(ctx, tconn, subtest.keySeq); err != nil {
				s.Fatalf("Failed to tap keys %v: %v", subtest.keySeq, err)
			}

			// some IMEs need to manually select from candidates to submit.
			if subtest.inputSuggestion {
				if err := vkb.SelectFromSuggestion(ctx, tconn, subtest.expectedText); err != nil {
					s.Fatalf("Failed to select %s from suggestions: %v", subtest.expectedText, err)
				}
			}

			if err := inputField.WaitForValueToBe(ctx, tconn, subtest.expectedText); err != nil {
				s.Fatal("Failed to verify input: ", err)
			}

			// Password input is a special case. The value is presented with placeholder "•".
			// Using PasswordTextField field to verify the outcome.
			if inputField == testserver.PasswordInputField {
				if err := testserver.PasswordTextField.WaitForValueToBe(ctx, tconn, strings.Join(subtest.keySeq[:], "")); err != nil {
					s.Fatal("Failed to verify password input: ", err)
				}
			}
		})
	}
}
