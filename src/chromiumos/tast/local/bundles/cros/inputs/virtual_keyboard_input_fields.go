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
				Val:               ime.XKB_US_ENG,
				ExtraHardwareDeps: pre.InputsStableModels,
			}, {
				Name:              "us_en_unstable",
				Val:               ime.XKB_US_ENG,
				ExtraHardwareDeps: pre.InputsUnstableModels,
			},
			{
				Name:              "jp_us_stable",
				Val:               ime.NACL_MOZC_US,
				ExtraHardwareDeps: pre.InputsStableModels,
			}, {
				Name:              "jp_us_unstable",
				Val:               ime.NACL_MOZC_US,
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
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
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
		testing.Sleep(ctx, 10*time.Second)
	}

	its, err := testserver.LaunchServer(ctx, cr, s.DataFileSystem())
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	type testData struct {
		inputField   testserver.InputField
		keySeq       []string
		expectedText string
	}

	var subTests []testData

	switch s.Param().(ime.InputMethodCode) {
	case ime.XKB_US_ENG:
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
	case ime.NACL_MOZC_US:
		subTests = []testData{
			{
				inputField:   testserver.TextAreaInputField,
				keySeq:       strings.Split("konnitiha", ""),
				expectedText: "こんいちは",
			}, {
				inputField:   testserver.TextInputField,
				keySeq:       strings.Split("konnitiha", ""),
				expectedText: "こんいちは",
			}, {
				inputField:   testserver.SearchInputField,
				keySeq:       strings.Split("konnitiha", ""),
				expectedText: "こんいちは",
			}, {
				inputField:   testserver.PasswordInputField,
				keySeq:       strings.Split("konnitiha", ""),
				expectedText: "•••••••••",
			}, {
				inputField:   testserver.NumberInputField,
				keySeq:       strings.Split("-123.456", ""),
				expectedText: "-123.456",
			}, {
				inputField:   testserver.EmailInputField,
				keySeq:       strings.Split("konnitiha", ""),
				expectedText: "こんいちは",
			}, {
				inputField:   testserver.URLInputField,
				keySeq:       strings.Split("konnitiha", ""),
				expectedText: "こんいちは",
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

			vkb.TapKeys(ctx, tconn, subtest.keySeq)

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
