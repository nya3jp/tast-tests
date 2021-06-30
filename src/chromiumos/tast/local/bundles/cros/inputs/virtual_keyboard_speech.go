// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

	// Setup CRAS Aloop for audio test.
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

	subtest := func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {

			if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
				s.Fatal("Failed to show VK: ", err)
			}

			defer func() {
				if err := vkbCtx.SwitchFromVoiceToTyping()(ctx); err != nil {
					s.Log("Failed to switch back to typing layout: ", err)
				}
				if err := its.Clear(inputField)(ctx); err != nil {
					s.Log("Failed to clear input field: ", err)
				}

				// Wait for IME to detect text change in the input field.
				testing.Sleep(ctx, time.Second)

				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}()

			if err := uiauto.Combine("verify audio input",
				vkbCtx.SwitchToVoiceInput(),
				func(ctx context.Context) error {
					return voice.AudioFromFile(ctx, s.DataPath(inputData.VoiceFile))
				},
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputData.ExpectedText),
				vkbCtx.SwitchFromVoiceToTyping(),
			)(ctx); err != nil {
				s.Fatal("Failed to validate voice input: ", err)
			}
		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, tconn, s, voiceTestIMEs, voiceTestMessages, subtest)
}
