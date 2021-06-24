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
	CheckBox                     []a11y.SpeechExpectation
	CheckBoxWithStateDescription []a11y.SpeechExpectation
	SeekBar                      []a11y.SpeechExpectation
	Slider                       []a11y.SpeechExpectation
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
				CheckBox: []a11y.SpeechExpectation{
					a11y.NewSpeechExpectation("CheckBox", false),
					a11y.NewSpeechExpectation("Check box", false),
					a11y.NewSpeechExpectation("Not checked", false),
					a11y.NewSpeechExpectation("Press Search plus Space to toggle", false),
				},
				CheckBoxWithStateDescription: []a11y.SpeechExpectation{
					a11y.NewSpeechExpectation("CheckBoxWithStateDescription", false),
					a11y.NewSpeechExpectation("Check box", false),
					a11y.NewSpeechExpectation("Not checked", false),
					a11y.NewSpeechExpectation("Press Search plus Space to toggle", false),
				},
				SeekBar: []a11y.SpeechExpectation{
					a11y.NewSpeechExpectation("seekBar", false),
					a11y.NewSpeechExpectation("Slider", false),
					a11y.NewSpeechExpectation("25", false),
					a11y.NewSpeechExpectation("Min 0", false),
					a11y.NewSpeechExpectation("Max 100", false),
				},
				Slider: []a11y.SpeechExpectation{
					a11y.NewSpeechExpectation("Slider", false),
					a11y.NewSpeechExpectation("3", false),
					a11y.NewSpeechExpectation("Min 0", false),
					a11y.NewSpeechExpectation("Max 10", false),
				},
			},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "vm",
			Val: expectedSpeechLog{
				CheckBox: []a11y.SpeechExpectation{
					a11y.NewSpeechExpectation("CheckBox", false),
					a11y.NewSpeechExpectation("Check box", false),
					a11y.NewSpeechExpectation("not checked", false),
					a11y.NewSpeechExpectation("Press Search plus Space to toggle", false),
				},
				CheckBoxWithStateDescription: []a11y.SpeechExpectation{
					a11y.NewSpeechExpectation("CheckBoxWithStateDescription", false),
					a11y.NewSpeechExpectation("Check box", false),
					a11y.NewSpeechExpectation("state description not checked", false),
					a11y.NewSpeechExpectation("Press Search plus Space to toggle", false),
				},
				SeekBar: []a11y.SpeechExpectation{
					a11y.NewSpeechExpectation("seekBar", false),
					a11y.NewSpeechExpectation("Slider", false),
					a11y.NewSpeechExpectation("state description 25", false),
					a11y.NewSpeechExpectation("Min 0", false),
					a11y.NewSpeechExpectation("Max 100", false),
				},
				Slider: []a11y.SpeechExpectation{
					a11y.NewSpeechExpectation("Slider", false),
					a11y.NewSpeechExpectation("30 percent", false),
					a11y.NewSpeechExpectation("Min 0", false),
					a11y.NewSpeechExpectation("Max 10", false),
				},
			},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

type axSpeechTestStep struct {
	keys         string
	expectations []a11y.SpeechExpectation
}

func AccessibilitySpeech(ctx context.Context, s *testing.State) {
	// TODO(b:146844194): Add test for EditTextActivity.
	MainActivityTestSteps := []axSpeechTestStep{
		{
			"Search+Right",
			[]a11y.SpeechExpectation{a11y.NewSpeechExpectation("Main Activity", false)},
		}, {
			"Search+Right",
			[]a11y.SpeechExpectation{a11y.NewSpeechExpectation("OFF", false), a11y.NewSpeechExpectation("Toggle Button", false), a11y.NewSpeechExpectation("Not pressed", false), a11y.NewSpeechExpectation("Press Search plus Space to toggle", false)},
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
			[]a11y.SpeechExpectation{a11y.NewSpeechExpectation("ANNOUNCE", false), a11y.NewSpeechExpectation("Button", false), a11y.NewSpeechExpectation("Press Search plus Space to activate", false)},
		}, {
			"Search+Space",
			[]a11y.SpeechExpectation{a11y.NewSpeechExpectation("test announcement", false)},
		}, {
			"Search+Right",
			[]a11y.SpeechExpectation{a11y.NewSpeechExpectation("CLICK TO SHOW TOAST", false), a11y.NewSpeechExpectation("Button", false), a11y.NewSpeechExpectation("Press Search plus Space to activate", false)},
		}, {
			"Search+Space",
			[]a11y.SpeechExpectation{a11y.NewSpeechExpectation("test toast", false)},
		},
	}

	testActivities := []arca11y.TestActivity{arca11y.MainActivity}

	speechTestSteps := map[string][]axSpeechTestStep{
		arca11y.MainActivity.Name: MainActivityTestSteps,
	}

	testFunc := func(ctx context.Context, cvconn *a11y.ChromeVoxConn, tconn *chrome.TestConn, currentActivity arca11y.TestActivity) error {
		if err := a11y.SetTTSRate(ctx, tconn, 5.0); err != nil {
			s.Fatal("Failed to change TTS rate: ", err)
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
			if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, []string{testStep.keys}, testStep.expectations); err != nil {
				return errors.Wrapf(err, "failure on the step %+v", testStep)
			}
		}
		return nil
	}
	arca11y.RunTest(ctx, s, testActivities, testFunc)
}
