// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"strconv"
	"strings"
	"testing"

	"chromiumos/tast/common/genparams"
)

func TestAudioLoopbackCorrectnessParams(t *testing.T) {
	const (
		classTestOutputSine = `"org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity"`
	)
	type valMember struct {
		Key   string
		Value string
	}
	type val struct {
		ArcaudioTestParams   []valMember
		IncorrectSlicesLimit int
	}
	type paramData struct {
		Name              string
		Val               val
		ExtraHardwareDeps string
	}

	type testcase struct {
		Class           string
		SampleRate      int
		ChannelConfig   string
		PerformanceMode string
	}

	var testcases []testcase

	// Different sample rate for stereo
	for _, sampleRate := range []int{8000, 11025, 16000, 22050, 32000, 44100, 48000} {
		testcases = append(testcases, testcase{
			Class:           classTestOutputSine,
			SampleRate:      sampleRate,
			ChannelConfig:   `arcaudio.ChannelConfigOutStereo`,
			PerformanceMode: `arcaudio.PerformanceModeNone`,
		})
	}

	// Different performance mode
	testcases = append(testcases, testcase{
		Class:           classTestOutputSine,
		SampleRate:      48000,
		ChannelConfig:   `arcaudio.ChannelConfigOutStereo`,
		PerformanceMode: `arcaudio.PerformanceModePowerSaving`,
	})
	testcases = append(testcases, testcase{
		Class:           classTestOutputSine,
		SampleRate:      48000,
		ChannelConfig:   `arcaudio.ChannelConfigOutStereo`,
		PerformanceMode: `arcaudio.PerformanceModeLowLatency`,
	})

	generateName := func(tierName string, tc testcase) string {
		var name []string
		if tierName != "" {
			name = append(name, tierName)
		}
		name = append(name, "stereo")
		name = append(name, strconv.Itoa(tc.SampleRate))
		switch tc.PerformanceMode {
		case `arcaudio.PerformanceModeLowLatency`:
			name = append(name, "lowlatency")
		case `arcaudio.PerformanceModePowerSaving`:
			name = append(name, "powersaving")
		}
		return strings.Join(name, "_")
	}

	var params []paramData

	for _, tier := range []struct {
		name                 string
		hwdep                string
		incorrectSlicesLimit int
	}{{ // Normal device tier
		hwdep:                `hwdep.D(hwdep.SkipOnModel(lowPerformanceModel...))`,
		incorrectSlicesLimit: 25,
	}, { // Low performance device tier
		name:                 "lowperf",
		hwdep:                `hwdep.D(hwdep.Model(lowPerformanceModel...))`,
		incorrectSlicesLimit: 50,
	}} {
		for _, tc := range testcases {
			params = append(params, paramData{
				Name: generateName(tier.name, tc),
				Val: val{
					ArcaudioTestParams: []valMember{
						{
							Key:   "Class",
							Value: tc.Class,
						},
						{
							Key:   "SampleRate",
							Value: strconv.Itoa(tc.SampleRate),
						},
						{
							Key:   "ChannelConfig",
							Value: tc.ChannelConfig,
						},
						{
							Key:   "PerformanceMode",
							Value: tc.PerformanceMode,
						},
					},
					IncorrectSlicesLimit: tier.incorrectSlicesLimit,
				},
				ExtraHardwareDeps: tier.hwdep,
			})
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		ExtraHardwareDeps: {{ .ExtraHardwareDeps }},
		Val: audioLoopbackCorrectnessVal{
			arcaudioTestParams: arcaudio.TestParameters{
				{{ range .Val.ArcaudioTestParams }}{{ .Key }}: {{ .Value }},
				{{ end }}
			},
			incorrectSlicesLimit: {{ .Val.IncorrectSlicesLimit }},
		},
	},
	{{ end }}`, params)
	genparams.Ensure(t, "audio_loopback_correctness.go", code)
}
