// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type audioInfo struct {
	InputDeviceName  string `json:"input_device_name"`
	InputGain        int    `json:"input_gain"`
	InputMute        bool   `json:"input_mute"`
	OutputDeviceName string `json:"output_device_name"`
	OutputMute       bool   `json:"output_mute"`
	OutputVolume     int    `json:"output_volume"`
	SevereUnderruns  int    `json:"severe_underruns"`
	Underruns        int    `json:"underruns"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeAudioInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for audio info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func validateAudioData(audio *audioInfo) error {
	// Check "input_device_name" is not empty.
	if audio.InputDeviceName == "" {
		return errors.New("Failed. input_device_name field is empty")
	}

	// Check "input_gain" is integer and between [0, 100].
	if audio.InputGain < 0 || audio.InputGain > 100 {
		return errors.Errorf("Failed. input_gain is not in a legal range [0, 100]: %d", audio.InputGain)
	}

	// Check "output_device_name" is not empty.
	if audio.OutputDeviceName == "" {
		return errors.New("Failed. output_device_name field is empty")
	}

	// Check "output_volume" is integer and between [0, 100].
	if audio.OutputVolume < 0 || audio.OutputVolume > 100 {
		return errors.Errorf("Failed. output_volume is not in a legal range [0, 100]: %d", audio.OutputVolume)
	}

	// Check "severe_underruns" is positive integer or zero.
	if audio.SevereUnderruns < 0 {
		return errors.Errorf("Failed. severe_underruns is smaller than zero: %d", audio.SevereUnderruns)
	}

	// Check "underruns" is positive integer or zero.
	if audio.Underruns < 0 {
		return errors.Errorf("Failed. underruns is smaller than zero: %d", audio.Underruns)
	}

	return nil
}

func ProbeAudioInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryAudio}
	var audio audioInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &audio); err != nil {
		s.Fatal("Failed to get audio telemetry info: ", err)
	}

	if err := validateAudioData(&audio); err != nil {
		s.Fatalf("Failed to validate audio data, err [%v]", err)
	}
}
