// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var typingTestIMEs = []ime.InputMethodCode{
	ime.INPUTMETHOD_XKB_US_ENG,
	ime.INPUTMETHOD_NACL_MOZC_US,
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
	ime.INPUTMETHOD_XKB_US_INTL,
	ime.INPUTMETHOD_XKB_GB_EXTD_ENG,
	ime.INPUTMETHOD_XKB_ES_SPA,
	ime.INPUTMETHOD_XKB_SE_SWE,
	ime.INPUTMETHOD_XKB_CA_ENG,
	ime.INPUTMETHOD_XKB_JP_JPN,
	ime.INPUTMETHOD_NACL_MOZC_JP,
	ime.INPUTMETHOD_XKB_FR_FRA,
	ime.INPUTMETHOD_CANTONESE_CHINESE_TRADITIONAL,
	ime.INPUTMETHOD_CANGJIE87_CHINESE_TRADITIONAL,
	ime.INPUTMETHOD_HANGEUL_HANJA_KOREAN,
}

var typingTestMessages = []data.Message{data.TypingMessageHello}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingIME,
		Desc:         "Checks that virtual keyboard works in different input methods",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools-upstream", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabledTablet,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(len(typingTestIMEs)) * time.Duration(len(typingTestMessages)) * time.Minute,
	})
}

func VirtualKeyboardTypingIME(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()
	inputField := testserver.TextAreaNoCorrectionInputField
	subtest := func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
				s.Fatal("Failed to show VK: ", err)
			}

			defer func() {
				// Cleanup states.
				if err := its.Clear(inputField)(ctx); err != nil {
					s.Log("Failed to clear input field: ", err)
				}

				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}()

			if err := uiauto.Combine("validate vk input function on field "+string(inputField),
				func(ctx context.Context) error {
					if err := vkbCtx.TapKeysIgnoringCase(inputData.CharacterKeySeq)(ctx); err != nil {
						return errors.Wrapf(err, "failed to tap keys: %v", inputData.CharacterKeySeq)
					}
					if inputData.SubmitFromSuggestion {
						return vkbCtx.SelectFromSuggestion(inputData.ExpectedText)(ctx)
					}
					return nil
				},
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputData.ExpectedText),
			)(ctx); err != nil {
				s.Fatal("Failed to validate virtual keyboard input: ", err)
			}

		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, tconn, s, typingTestIMEs, typingTestMessages, subtest)
}
