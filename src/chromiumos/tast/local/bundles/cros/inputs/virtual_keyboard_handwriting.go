// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

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

// Documentation on file format can be found in go/tast-handwriting-svg-parsing.
const (
	handwritingWarmupFile  = "handwriting_digit_3_20210510.svg"
	handwritingWarmupDigit = "3"
)

var hwTestMessages = []data.Message{data.HandwritingMessageHello}
var hwTestIMEs = []ime.InputMethodCode{
	ime.INPUTMETHOD_NACL_MOZC_JP,
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
	ime.INPUTMETHOD_XKB_US_ENG,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardHandwriting,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		Data:         append(data.ExtractExternalFiles(hwTestMessages, hwTestIMEs), handwritingWarmupFile),
		Pre:          pre.VKEnabledTablet,
		Timeout:      time.Duration(len(hwTestIMEs)) * time.Duration(len(hwTestMessages)) * time.Minute,
		// kevin64 board doesn't support nacl, thus IMEs using nacl for handwriting canvas fail.
		// Have to exclude entire kevin model as no distinguish between kevin and kevin64.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kevin1")),
		Params: []testing.Param{
			{
				Name: "docked",
				// false for docked-mode VK
				Val: false,
			},
			{
				Name: "floating",
				// true for floating-mode VK
				Val: true,
			},
		},
	})
}

func VirtualKeyboardHandwriting(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	isFloating := s.Param().(bool)

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	// Creates subtest that runs the test logic using inputData.
	subtest := func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
				s.Fatal("Failed to show VK: ", err)
			}
			defer func() {
				// Cleanup states.
				if err := uiauto.Combine("clean up",
					its.Clear(inputField),
					vkbCtx.SwitchFromHandwritingToTyping(),
					vkbCtx.SwitchToDockedMode(),
					vkbCtx.HideVirtualKeyboard(),
				)(ctx); err != nil {
					s.Log("Failed to clean up: ", err)
				}

				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)
			}()

			swtichAction := vkbCtx.SwitchToDockedMode()
			if isFloating {
				swtichAction = vkbCtx.SwitchToFloatingMode()
			}
			if err := swtichAction(ctx); err != nil {
				s.Fatal("Failed to set VK floating mode: ", err)
			}

			// Switch to handwriting layout.
			hwCtx, err := vkbCtx.SwitchToHandwritingAndCloseInfoDialogue(ctx)
			if err != nil {
				s.Fatal("Failed to switch to handwriting: ", err)
			}

			// Warm-up steps to check handwriting engine ready.
			checkEngineReady := uiauto.Combine("Wait for handwriting engine to be ready",
				hwCtx.DrawStrokesFromFile(s.DataPath(handwritingWarmupFile)),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), handwritingWarmupDigit),
				hwCtx.ClearHandwritingCanvas(),
				its.Clear(inputField))

			if err := uiauto.Combine("Test handwriting on virtual keyboard",
				hwCtx.WaitForHandwritingEngineReady(checkEngineReady),
				hwCtx.DrawStrokesFromFile(s.DataPath(inputData.HandwritingFile)),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), inputData.ExpectedText),
			)(ctx); err != nil {
				s.Fatal("Failed to verify handwriting input: ", err)
			}
		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, tconn, s, hwTestIMEs, hwTestMessages, subtest)
}
