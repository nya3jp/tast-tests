// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input/voice"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var voiceTestMessages = []data.Message{data.VoiceMessageHello}
var voiceTestIMEs = []ime.InputMethod{
	ime.ChinesePinyin,
	ime.EnglishUS,
}
var voiceTestIMEsNewData = []ime.InputMethod{
	ime.AlphanumericWithJapaneseKeyboard,
	ime.Arabic,
	ime.EnglishUK,
	ime.EnglishUSWithInternationalKeyboard,
	ime.Japanese,
	ime.Korean,
	ime.EnglishSouthAfrica,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSpeech,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test voice input functionality on virtual keyboard",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		SearchFlags:  util.IMESearchFlags(voiceTestIMEs),
		Data:         data.ExtractExternalFiles(voiceTestMessages, append(voiceTestIMEs, voiceTestIMEsNewData...)),
		Timeout:      time.Duration(len(voiceTestIMEs)+len(voiceTestIMEsNewData)) * time.Duration(len(voiceTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Val:               voiceTestIMEs,
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "newdata", // This test will be merged into CQ once it is proved to be stable.
				Fixture:           fixture.TabletVK,
				Val:               voiceTestIMEsNewData,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
				ExtraSearchFlags:  util.IMESearchFlags(voiceTestIMEsNewData),
			},
			{
				Name:              "informational",
				Fixture:           fixture.TabletVK,
				Val:               append(voiceTestIMEs, voiceTestIMEsNewData...),
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosTabletVK,
				Val:               append(voiceTestIMEs, voiceTestIMEsNewData...),
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraSearchFlags:  util.IMESearchFlags(voiceTestIMEsNewData),
			},
		},
	})
}

func VirtualKeyboardSpeech(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	testIMEs := s.Param().([]ime.InputMethod)

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup CRAS Aloop for audio test.
	cleanup, err := voice.EnableAloop(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to load Aloop: ", err)
	}
	defer cleanup(cleanupCtx)

	// Launch inputs test web server.
	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

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

			verifyAudioInputAction := uiauto.Combine("verify audio input",
				its.Clear(inputField),
				uiauto.Sleep(time.Second),
				its.ClickFieldUntilVKShown(inputField),
				vkbCtx.SwitchToVoiceInput(),
				func(ctx context.Context) error {
					return voice.AudioFromFile(ctx, s.DataPath(inputData.VoiceFile))
				},
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputData.ExpectedText),
			)

			if err := uiauto.UserAction("Voice input",
				verifyAudioInputAction,
				uc,
				&useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeInputField: string(inputField),
						useractions.AttributeFeature:    useractions.FeatureVoiceInput,
					},
				},
			)(ctx); err != nil {
				s.Fatal("Failed to validate voice input: ", err)
			}
		}
	}
	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, uc, s, testIMEs, voiceTestMessages, subtest)
}
