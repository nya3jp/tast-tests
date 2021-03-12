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
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	audioFileEN = "voice_en_hello_20201021.wav"
	audioFileCN = "voice_cn_hello_20201021.wav"
)

// Struct to contain the virtual keyboard speech test parameters.
type speechTestParams struct {
	audioFile    string
	expectedText string
	imeID        ime.InputMethodCode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSpeech,
		Desc:         "Test voice input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		Params: []testing.Param{
			{
				Name:      "hello_en",
				ExtraData: []string{audioFileEN},
				Val: speechTestParams{
					audioFile:    audioFileEN,
					expectedText: "Hello",
					imeID:        ime.INPUTMETHOD_XKB_US_ENG,
				},
			}, {
				Name:      "hello_cn",
				ExtraData: []string{audioFileCN},
				Val: speechTestParams{
					audioFile:    audioFileCN,
					expectedText: "你好",
					imeID:        ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
				},
			},
		},
	})
}

func VirtualKeyboardSpeech(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer shortCancel()

	// TODO(crbug/1173252): Clean up states within Chrome using preconditions.
	cr, err := chrome.New(ctx, chrome.VKEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	cleanup, err := input.EnableAloopInput(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to enable Aloop: ", err)
	}
	defer cleanup(cleanupCtx)

	// Test parameters that are specific to the current test case.
	audioFile := s.Param().(speechTestParams).audioFile
	expectedText := s.Param().(speechTestParams).expectedText
	testIME := ime.IMEPrefix + string(s.Param().(speechTestParams).imeID)

	// Set up the test audio file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, audioFile)
	if err := fsutil.CopyFile(s.DataPath(audioFile), testFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField

	// Open the virtual keyboard.
	if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	// Add the ime input being tested to the test device.
	if err := ime.AddAndSetInputMethod(ctx, tconn, testIME); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", testIME, err)
	}

	// Activate voice input.
	if err := vkb.SwitchToVoiceInput(ctx, cr); err != nil {
		s.Fatal("Failed to switch on voice input: ", err)
	}

	// Playback the audio into the voice input.
	if err := input.AudioFromFile(ctx, testFileLocation); err != nil {
		s.Fatal("Failed to input audio: ", err)
	}

	// Verify if the derived text is equal to the expected text.
	if err := its.WaitForFieldValueToBe(inputField, expectedText)(ctx); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
