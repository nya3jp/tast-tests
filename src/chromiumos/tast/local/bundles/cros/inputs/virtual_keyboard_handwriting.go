// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
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
	handwritingFileEN      = "handwriting_en_hello_20210129.svg"
	handwritingFileCN      = "handwriting_cn_hello_20210129.svg"
	handwritingFileJP      = "handwriting_jp_hello_20210129.svg"
)

// Struct to contain the virtual keyboard handwriting test parameters.
type handwritingTestParams struct {
	handwritingFile string
	expectedText    string
	imeID           ime.InputMethodCode
	testFloat       bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardHandwriting,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		Data:         []string{handwritingWarmupFile},
		// kevin64 board doesn't support nacl, thus IMEs using nacl for handwriting canvas fail.
		// Have to exclude entire kevin model as no distinguish between kevin and kevin64.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kevin1")),
		Params: []testing.Param{
			{
				Name:      "hello_jp",
				ExtraData: []string{handwritingFileJP},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileJP,
					expectedText:    "こんにちは",
					imeID:           ime.INPUTMETHOD_NACL_MOZC_JP,
				},
			}, {
				Name:      "hello_jp_float",
				ExtraData: []string{handwritingFileJP},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileJP,
					expectedText:    "こんにちは",
					imeID:           ime.INPUTMETHOD_NACL_MOZC_JP,
					testFloat:       true,
				},
			}, {
				Name:      "hello_cn",
				ExtraData: []string{handwritingFileCN},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileCN,
					expectedText:    "你好",
					imeID:           ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
				},
			}, {
				Name:      "hello_cn_float",
				ExtraData: []string{handwritingFileCN},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileCN,
					expectedText:    "你好",
					imeID:           ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
					testFloat:       true,
				},
			}, {
				Name:      "hello_en",
				ExtraData: []string{handwritingFileEN},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileEN,
					expectedText:    "hello",
					imeID:           ime.INPUTMETHOD_XKB_US_ENG,
				},
			}, {
				Name:      "hello_en_float",
				ExtraData: []string{handwritingFileEN},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileEN,
					expectedText:    "hello",
					imeID:           ime.INPUTMETHOD_XKB_US_ENG,
					testFloat:       true,
				},
			},
		},
	})
}

func VirtualKeyboardHandwriting(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Variable to contain the test parameters that are specific to the current test case.
	params := s.Param().(handwritingTestParams)

	// Options containing preconditions.
	opts := []chrome.Option{
		chrome.VKEnabled(),
		chrome.ExtraArgs("--force-tablet-mode=touch_view"),
	}

	// Add precondition of requiring a floating keyboard if testing for floating handwriting input.
	if params.testFloat {
		opts = append(opts, chrome.EnableFeatures("VirtualKeyboardFloatingDefault"))
	}

	// TODO(crbug/1173252): Clean up states within Chrome using preconditions.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to connect to new Chrome instance: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// IME code of the language currently being tested.
	testIME := ime.IMEPrefix + string(params.imeID)

	// Add and set the required ime for the test case.
	if err := ime.AddAndSetInputMethod(ctx, tconn, testIME); err != nil {
		s.Fatalf("Failed to set input method to %s: %v", testIME, err)
	}

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	// Show VK.
	if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
		s.Fatal("Failed to show VK: ", err)
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
		hwCtx.DrawStrokesFromFile(s.DataPath(params.handwritingFile)),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), params.expectedText),
	)(ctx); err != nil {
		s.Fatal("Failed to verify handwriting input: ", err)
	}
}
