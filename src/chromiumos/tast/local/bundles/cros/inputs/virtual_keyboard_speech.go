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
	audioFileEn = "voice_en_hello_20201021.wav"
	audioFileCn = "voice_cn_hello_20201021.wav"
)

// struct to contain the virtual keyboard speech test parameters
type vksTestParams struct {
	audioFile    string
	expectedText string
	imeID        string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSpeech,
		Desc:         "Test voice input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		// This test is a technical experiment. It is very flaky at the moment.
		// Attr:         []string{"group:mainline", "informational", "group:essential-inputs"}
		Pre: pre.VKEnabledTablet,
		Params: []testing.Param{
			{
				Name:      "hello_en",
				ExtraData: []string{audioFileEn},
				Val: vksTestParams{
					audioFile:    audioFileEn,
					expectedText: "hello",
					imeID:        string(ime.INPUTMETHOD_XKB_US_ENG),
				},
			}, {
				Name:      "hello_cn",
				ExtraData: []string{audioFileCn},
				Val: vksTestParams{
					audioFile:    audioFileCn,
					expectedText: "你好",
					imeID:        string(ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED),
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

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	cleanup, err := input.EnableAloopInput(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to enable Aloop: ", err)
	}
	defer cleanup(cleanupCtx)

	// Get the audio file name
	audioFile := s.Param().(vksTestParams).audioFile

	// Set up the test audio file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, audioFile)
	if err := fsutil.CopyFile(s.DataPath(audioFile), testFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Launch inputs test web server.
	ts, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer ts.Close()

	// Select the input field
	inputField := testserver.TextAreaInputField

	// Open the virtual keyboard
	if err := inputField.ClickUntilVKShown(ctx, tconn); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}
	defer vkb.HideVirtualKeyboard(ctx, tconn)

	// Get the ime code
	testIME := ime.IMEPrefix + s.Param().(vksTestParams).imeID

	// Get the current ime code
	currentIME, err := ime.GetCurrentInputMethod(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current ime: ", err)
	}

	// Only install input when the current ime is different to the ime we want
	if testIME != currentIME {
		// Add the ime input being tested to the test device
		if err := ime.AddAndSetInputMethod(ctx, tconn, testIME); err != nil {
			s.Fatalf("Failed to set input method to %s: %v: ", testIME, err)
		}
	}

	// Activate voice input
	if err := vkb.SwitchToVoiceInput(ctx, tconn); err != nil {
		s.Fatal("Failed to switch on voice input: ", err)
	}

	// Playback the audio into the voice input
	if err := input.AudioFromFile(ctx, testFileLocation); err != nil {
		s.Fatal("Failed to input audio: ", err)
	}

	// Get the expected text
	expectedText := s.Param().(vksTestParams).expectedText

	// Verify if the derived text is equal to the expected text
	if err := inputField.WaitForValueToBe(ctx, tconn, expectedText); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
