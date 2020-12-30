// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/input"
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
		// Attr:         []string{"group:mainline", "informational", "group:essential-inputs"}
		Pre: pre.VKEnabledTablet,
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

	cleanup, err := input.EnableAloopInput(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to enable Aloop: ", err)
	}
	defer cleanup(cleanupCtx)

	// Test parameters that are specific to the current test case.
	handwritingFile := s.Param().(handwritingTestParams).handwritingFile
	expectedText := s.Param().(handwritingTestParams).expectedText
	testIME := ime.IMEPrefix + string(s.Param().(handwritingTestParams).imeID)

	// Set up the test handwriting file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, handwritingFile)
	if err := fsutil.CopyFile(s.DataPath(handwritingFile), testFileLocation); err != nil {
		s.Fatalf("Failed to copy the test handwriting file to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

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
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	// Get the current ime code.
	currentIME, err := ime.GetCurrentInputMethod(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current ime: ", err)
	}

	// Only install input when the current ime is different to the ime we are testing.
	if testIME != currentIME {
		// Add the ime input being tested to the test device.
		if err := ime.AddAndSetInputMethod(ctx, tconn, testIME); err != nil {
			s.Fatalf("Failed to set input method to %s: %v: ", testIME, err)
		}
	}

	// Set the default layout to keyboard input.
	if err := vkb.TapKeyboardInput(ctx, tconn); err != nil {
		s.Log("Failed to tap keyboard input: ", err)
	}

	// Show the controls for handwriting input.
	if err := vkb.TapAccessPoints(ctx, tconn); err != nil {
		s.Log("Failed to tap access points: ", err)
	}

	// Activate voice input.
	if err := vkb.TapHandwritingInput(ctx, tconn); err != nil {
		s.Fatal("Failed to tap handwriting input: ", err)
	}

	// Read and populate the data from the handwriting strokes file.
	pathGroup, err := vkb.ReadFileAndPopulateData(ctx, tconn, testFileLocation)
	if err != nil {
		s.Fatal("Failed to read and populate data: ", err)
	}

	// Draw on the canvas using the populated data.
	if err := vkb.DrawHandwriting(ctx, tconn, pathGroup); err != nil {
		s.Fatal("Failed to draw on canvas: ", err)
	}

	// Verify if the derived text is equal to the expected text.
	if err := inputField.WaitForValueToBe(ctx, tconn, expectedText); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
