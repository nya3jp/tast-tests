// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
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

var typingModeTestIMEs = []ime.InputMethodCode{
	ime.INPUTMETHOD_XKB_US_ENG,
	ime.INPUTMETHOD_XKB_JP_JPN,
}
var typingModeTestMessages = []data.Message{data.TypingMessageHello}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingUserMode,
		Desc:         "Checks that virtual keyboard works in different user modes",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools-upstream", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "guest",
			Pre:  pre.VKEnabledInGuest,
		},
			{Name: "incognito",
				Pre: pre.VKEnabledReset},
		},
	})
}

func VirtualKeyboardTypingUserMode(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	its, err := testserver.LaunchInMode(ctx, cr, tconn, s.TestName() == "incognito")
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	subtest := func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			// Use a shortened context for test operations to reserve time for cleanup.
			ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer shortCancel()

			defer func(ctx context.Context) {

				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}(cleanupCtx)

			if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
				s.Fatal("Failed to show VK: ", err)
			}

			if err := uiauto.Combine("validate vk input function on field "+string(inputField),
				its.Clear(inputField),
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
	util.RunSubtestsPerInputMethodAndMessage(ctx, tconn, s, typingModeTestIMEs, typingModeTestMessages, subtest)
}
