// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

// TODO(crbug/1164089): Create documentation for handwriting file format.
const (
	handwritingFileEN = "handwriting_en_hello_20210108.hw"
	handwritingFileCN = "handwriting_cn_hello_20210108.hw"
	handwritingFileJP = "handwriting_jp_hello_20210108.hw"
)

// Struct to contain the virtual keyboard handwriting test parameters.
type handwritingTestParams struct {
	handwritingFile string
	expectedText    string
	imeID           ime.InputMethodCode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardHandwriting,
		Desc:         "Test handwriting input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:input-tools"},
		Pre:          pre.VKEnabledTablet,
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
				Name:      "hello_cn",
				ExtraData: []string{handwritingFileCN},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileCN,
					expectedText:    "你好",
					imeID:           ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
				},
			}, {
				Name:      "hello_en",
				ExtraData: []string{handwritingFileEN},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileEN,
					expectedText:    "hello",
					imeID:           ime.INPUTMETHOD_XKB_US_ENG,
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

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Variable to contain the test parameters that are specific to the current test case.
	params := s.Param().(handwritingTestParams)

	// IME code of the language currently being tested.
	testIME := ime.IMEPrefix + string(params.imeID)

	// Launch inputs test web server.
	ts, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer ts.Close()

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField

	// Open the virtual keyboard.
	if err := inputField.ClickUntilVKShown(ctx, tconn); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}

	// Add and set the required ime for the test case.
	if err := ime.AddAndSetInputMethod(ctx, tconn, testIME); err != nil {
		s.Fatalf("Failed to set input method to %s: %v", testIME, err)
	}

	// Activate handwriting input.
	if err := vkb.TapHandwritingInputAndWaitForEngine(ctx, tconn); err != nil {
		s.Fatal("Failed to tap handwriting input: ", err)
	}
	// TapKeyboardInput is used to reset the keyboard state back to keyboard input.
	defer vkb.TapKeyboardInput(cleanupCtx, tconn)
	// TapAccessPoints is used to show the keyboard layout buttons.
	defer vkb.TapAccessPoints(cleanupCtx, tconn)

	// Read and populate the data from the handwriting strokes file, then draw on the canvas.
	if err := vkb.DrawHandwritingFromFile(ctx, tconn, s.DataPath(params.handwritingFile)); err != nil {
		s.Fatal("Failed to draw handwriting from file: ", err)
	}

	// Verify if the derived text is equal to the expected text.
	if err := inputField.WaitForValueToBe(ctx, tconn, params.expectedText); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}

}
