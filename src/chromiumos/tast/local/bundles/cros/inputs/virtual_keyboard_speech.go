// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/input/voice"
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

var voiceTestMessages = []data.Message{data.VoiceMessageHello}
var voiceTestIMEs = []ime.InputMethodCode{
	ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
	ime.INPUTMETHOD_XKB_US_ENG,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSpeech,
		Desc:         "Test voice input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		Data:         data.ExtractExternalFiles(voiceTestMessages, voiceTestIMEs),
		Pre:          pre.VKEnabledTablet,
		Timeout:      time.Duration(len(voiceTestIMEs)) * time.Duration(len(voiceTestMessages)) * time.Minute,
	})
}

func VirtualKeyboardSpeech(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := voice.EnableAloop(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to load Aloop: ", err)
	}
	defer cleanup(ctx)

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	for _, inputMethod := range voiceTestIMEs {
		for _, message := range voiceTestMessages {
			inputData, ok := message.GetInputData(inputMethod)
			if !ok {
				s.Fatalf("Test Data for input method %v does not exist", inputMethod)
			}
			testName := string(inputMethod) + "-" + string(inputData.ExpectedText)
			s.Run(ctx, testName, func(ctx context.Context, subS *testing.State) {
				// IME code of the language currently being tested.
				testIME := ime.IMEPrefix + string(inputMethod)

				// Set up the test audio file.
				testFileLocation := filepath.Join(filesapp.DownloadPath, inputData.VoiceFile)
				if err := fsutil.CopyFile(s.DataPath(inputData.VoiceFile), testFileLocation); err != nil {
					s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
				}
				defer os.Remove(testFileLocation)

				// Add the ime input being tested to the test device.
				if err := ime.AddAndSetInputMethod(ctx, tconn, testIME); err != nil {
					s.Fatalf("Failed to set input method to %s: %v: ", testIME, err)
				}

				if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
					s.Fatal("Failed to show VK: ", err)
				}

				defer func() {
					if err := vkbCtx.ClearInputField(inputField.Finder())(ctx); err != nil {
						s.Log("Failed to clear input field: ", err)
					}

					outDir := filepath.Join(s.OutDir(), testName)
					faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

					if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
						s.Log("Failed to hide virtual keyboard: ", err)
					}
				}()

				if err := uiauto.Combine("verify audio input",
					vkbCtx.SwitchToVoiceInput(),
					func(ctx context.Context) error {
						return voice.AudioFromFile(ctx, testFileLocation)
					},
					util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputData.ExpectedText),
					vkbCtx.SwitchFromVoiceToTyping(),
				)(ctx); err != nil {
					s.Fatal("Failed to validate voice input: ", err)
				}
			})
		}
	}
}
