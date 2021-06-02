// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/arc"
	arca11y "chromiumos/tast/local/bundles/cros/arc/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type expectedSpeechLog struct {
	CheckBox                     []string
	CheckBoxWithStateDescription []string
	SeekBar                      []string
	Slider                       []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilitySpeech,
		Desc:         "Checks ChromeVox reads Android elements as expected",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Val: expectedSpeechLog{
				CheckBox: []string{
					"CheckBox", "Check box", "Not checked", "Press Search plus Space to toggle",
				},
				CheckBoxWithStateDescription: []string{
					"CheckBoxWithStateDescription", "Check box", "Not checked", "Press Search plus Space to toggle",
				},
				SeekBar: []string{
					"seekBar", "Slider", "25", "Min 0", "Max 100",
				},
				Slider: []string{
					"Slider", "3", "Min 0", "Max 10",
				},
			},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "vm",
			Val: expectedSpeechLog{
				CheckBox: []string{
					"CheckBox", "Check box", "not checked", "Press Search plus Space to toggle",
				},
				CheckBoxWithStateDescription: []string{
					"CheckBoxWithStateDescription", "Check box", "state description not checked", "Press Search plus Space to toggle",
				},
				SeekBar: []string{
					"seekBar", "Slider", "state description 25", "Min 0", "Max 100",
				},
				Slider: []string{
					"Slider", "30 percent", "Min 0", "Max 10",
				},
			},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

type axSpeechTestStep struct {
	keys       string
	utterances []string
}

func AccessibilitySpeech(ctx context.Context, s *testing.State) {
	// TODO(b:146844194): Add test for EditTextActivity.
	MainActivityTestSteps := []axSpeechTestStep{
		{
			"Search+Right",
			[]string{"Main Activity"},
		}, {
			"Search+Right",
			[]string{"OFF", "Toggle Button", "Not pressed", "Press Search plus Space to toggle"},
		}, {
			"Search+Right",
			s.Param().(expectedSpeechLog).CheckBox,
		}, {
			"Search+Right",
			s.Param().(expectedSpeechLog).CheckBoxWithStateDescription,
		}, {
			"Search+Right",
			s.Param().(expectedSpeechLog).SeekBar,
		}, {
			"Search+Right",
			s.Param().(expectedSpeechLog).Slider,
		}, {
			"Search+Right",
			[]string{"ANNOUNCE", "Button", "Press Search plus Space to activate"},
		}, {
			"Search+Space",
			[]string{"test announcement"},
		}, {
			"Search+Right",
			[]string{"CLICK TO SHOW TOAST", "Button", "Press Search plus Space to activate"},
		}, {
			"Search+Space",
			[]string{"test toast"},
		},
	}

	testActivities := []arca11y.TestActivity{arca11y.MainActivity}

	speechTestSteps := map[string][]axSpeechTestStep{
		arca11y.MainActivity.Name: MainActivityTestSteps,
	}

	testFunc := func(ctx context.Context, cvconn *a11y.ChromeVoxConn, tconn *chrome.TestConn, currentActivity arca11y.TestActivity) error {
		if err := a11y.SetTTSRate(ctx, tconn, 5.0); err != nil {
			s.Fatal("Faild to change TTS rate: ", err)
		}
		defer a11y.SetTTSRate(ctx, tconn, 1.0)

		if err := cvconn.SetVoice(ctx, a11y.VoiceData{
			ExtID:  a11y.GoogleTTSExtensionID,
			Locale: "en-US",
		}); err != nil {
			return errors.Wrap(err, "failed to set the ChromeVox voice")
		}

		sm, err := a11y.RelevantSpeechMonitor(ctx, s.FixtValue().(*arc.PreData).Chrome, tconn, a11y.TTSEngineData{ExtID: a11y.GoogleTTSExtensionID, UseOnSpeakWithAudioStream: false})
		if err != nil {
			return errors.Wrap(err, "failed to connect to the TTS background page")
		}
		defer sm.Close()

		testSteps := speechTestSteps[currentActivity.Name]
		for _, testStep := range testSteps {
			if err := a11y.PressKeysAndConsumeUtterances(ctx, sm, []string{testStep.keys}, testStep.utterances); err != nil {
				return errors.Wrapf(err, "failure on the step %+v", testStep)
			}
		}
		return nil
	}
	arca11y.RunTest(ctx, s, testActivities, testFunc)
}
