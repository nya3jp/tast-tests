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
	"chromiumos/tast/testing"
)

const (
	handwritingFileEN = "handwriting_en_hello_20201216.txt"
)

// Struct to contain the virtual keyboard speech test parameters.
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
		Pre:          pre.VKEnabledTablet,
		Params: []testing.Param{
			{
				Name:      "hello_en",
				ExtraData: []string{handwritingFileEN},
				Val: handwritingTestParams{
					handwritingFile: handwritingFileEN,
					expectedText:    "Hello",
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

	// Set up the input handwriting file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, handwritingFile)
	if err := fsutil.CopyFile(s.DataPath(handwritingFile), testFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Read handwiting trajactories from file.
	pathGroup, err := vkb.ReadHandwritingFile(ctx, tconn, testFileLocation)
	if err != nil {
		s.Fatal("Failed to read handwriting file: ", err)
	}

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

	// Click the access point to switch from zero state sugguestion to input
	// option buttons including handwriting button.
	if err := vkb.ClickAccessPoint(ctx, tconn); err != nil {
		s.Fatal("Failed to click access points: ", err)
	}

	// Click the handwriting button to enable the handwriting canvas.
	if err := vkb.SwitchToHandwritingInput(ctx, tconn); err != nil {
		s.Fatal("Failed to swtich to handwriting: ", err)
	}

	handwritingCanvas, err := vkb.FindHandwritingCanvas(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get handwriting canvas: ", err)
	}

	if err := vkb.DrawHandwritingOnCanvas(ctx, tconn, pathGroup, handwritingCanvas.Location); err != nil {
		s.Fatal("Failed to draw handwriting on canvas: ", err)
	}

	// Verify if the derived text is equal to the expected text.
	if err := inputField.WaitForValueToBe(ctx, tconn, expectedText); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
