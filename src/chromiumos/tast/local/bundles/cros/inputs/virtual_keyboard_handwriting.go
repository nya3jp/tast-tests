// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

const (
	handwritingFileEN = "handwriting_en_hello.hw"
	handwritingFileCN = "handwriting_cn_hello.hw"
	handwritingFileJP = "handwriting_jp_hello.hw"
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
	ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer shortCancel()

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Test parameters that are specific to the current test case.
	handwritingFile := s.Param().(handwritingTestParams).handwritingFile
	expectedText := s.Param().(handwritingTestParams).expectedText
	testIME := ime.IMEPrefix + string(s.Param().(handwritingTestParams).imeID)

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
	if err := vkb.TapHandwritingInput(ctx, tconn); err != nil {
		s.Fatal("Failed to tap handwriting input: ", err)
	}
	// TapKeyboardInput is used to reset the keyboard state back to keyboard input.
	defer vkb.TapKeyboardInput(ctx, tconn)
	// TapAccessPoints is used to show the keyboard layout buttons.
	defer vkb.TapAccessPoints(ctx, tconn)

	// Read and populate the data from the handwriting strokes file, then draw on the canvas.
	if err := vkb.DrawHandwritingFromFile(ctx, tconn, s.DataPath(handwritingFile)); err != nil {
		s.Fatal("Failed to read and populate data: ", err)
	}

	// Verify if the derived text is equal to the expected text.
	if err := inputField.WaitForValueToBe(ctx, tconn, expectedText); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}

}
