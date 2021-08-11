// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
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
	"chromiumos/tast/testing/hwdep"
)

var voiceTestMessages = []data.Message{data.VoiceMessageHello}
var voiceTestIMEs = []ime.InputMethod{
	ime.ChinesePinyin,
	ime.EnglishUS,
}
var voiceTestIMEsExtra = []ime.InputMethod{
	ime.AlphanumericWithJapaneseKeyboard,
	ime.Arabic,
	ime.EnglishUK,
	ime.EnglishUSWithInternationalKeyboard,
	ime.Japanese,
	ime.Korean,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSpeech,
		Desc:         "Test voice input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		Data:         data.ExtractExternalFiles(voiceTestMessages, voiceTestIMEs),
		Pre:          pre.VKEnabledReset,
		Timeout:      time.Duration(len(voiceTestIMEs)+len(voiceTestIMEsExtra)) * time.Duration(len(voiceTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Val:               voiceTestIMEs,
			},
			{
				Name:              "newdata", // This test will be merged into CQ once it is proved to be stable.
				Val:               voiceTestIMEsExtra,
				ExtraData:         data.ExtractExternalFiles(voiceTestMessages, voiceTestIMEsExtra),
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "informational",
				Val:               voiceTestIMEs,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func VirtualKeyboardSpeech(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	testIMEs := s.Param().([]ime.InputMethod)

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
			cleanupCtx := ctx
			// Use a shortened context for test operations to reserve time for cleanup.
			ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer shortCancel()

			defer func(ctx context.Context) {

				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}(cleanupCtx)

			if err := uiauto.Combine("verify audio input",
				its.Clear(inputField),
				uiauto.New(tconn).Sleep(time.Second),
				its.ClickFieldUntilVKShown(inputField),
				vkbCtx.SwitchToVoiceInput(),
				func(ctx context.Context) error {
					return voice.AudioFromFile(ctx, s.DataPath(inputData.VoiceFile))
				},
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputData.ExpectedText),
			)(ctx); err != nil {
				s.Fatal("Failed to validate voice input: ", err)
			}
		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, tconn, s, testIMEs, voiceTestMessages, subtest)
}
