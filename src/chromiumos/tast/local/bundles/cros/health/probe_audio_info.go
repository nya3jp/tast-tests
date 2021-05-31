// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"reflect"
	"strconv"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

const (
	audioOutputMute       = "output_mute"
	audioInputMute        = "input_mute"
	audioOutputVolume     = "output_volume"
	audioOutputDeviceName = "output_device_name"
	audioInputGain        = "input_gain"
	audioInputDeviceName  = "input_device_name"
	audioUnderruns        = "underruns"
	audioSevereUnderruns  = "severe_underruns"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeAudioInfo,
		Desc: "Check that we can probe cros_healthd for audio info",
		Contacts: []string{
			"kerker@google.com",
			"cros-tdm@google.com",
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeAudioInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryAudio}
	records, err := croshealthd.RunAndParseTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get audio telemetry info: ", err)
	}

	if len(records) != 2 {
		s.Fatalf("Wrong number of records: got %d; want 2", len(records))
	}

	// Verify the headers are correct.
	want := []string{
		audioOutputMute,
		audioInputMute,
		audioOutputDeviceName,
		audioOutputVolume,
		audioInputDeviceName,
		audioInputGain,
		audioUnderruns,
		audioSevereUnderruns,
	}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify the amount of values is correct.
	vals := records[1]
	if len(vals) != len(want) {
		s.Fatalf("Wrong number of values: got %d; want %d", len(vals), len(want))
	}

	// Using a map, then we don't need to take care of the index change in future.
	contentsMap := make(map[string]string)
	for i, elem := range want {
		contentsMap[elem] = vals[i]
	}

	// Check "output_mute" and "input_mute" is bool.
	if _, err := strconv.ParseBool(contentsMap[audioOutputMute]); err != nil {
		s.Errorf("Failed to convert %q (%s) to bool: %v", contentsMap[audioOutputMute], audioOutputMute, err)
	}
	if _, err := strconv.ParseBool(contentsMap[audioInputMute]); err != nil {
		s.Errorf("Failed to convert %q (%s) to bool: %v", contentsMap[audioInputMute], audioInputMute, err)
	}

	// Check "output_device_name" is not empty.
	if contentsMap[audioOutputDeviceName] == "" {
		s.Errorf("Failed. %s field is empty", audioOutputDeviceName)
	}

	// Check "output_volume" is integer and between [0, 100].
	outputVolume, err := strconv.ParseInt(contentsMap[audioOutputVolume], 10, 32)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to int: %v", contentsMap[audioOutputVolume], audioOutputVolume, err)
	}
	if outputVolume < 0 || outputVolume > 100 {
		s.Error("Failed. output_volume is not in a legal range [0, 100]: ", outputVolume)
	}

	// Check "input_device_name" is not empty.
	if contentsMap[audioInputDeviceName] == "" {
		s.Errorf("Failed. %s field is empty", audioInputDeviceName)
	}

	// Check "input_gain" is integer and between [0, 100].
	inputGain, err := strconv.ParseInt(contentsMap[audioInputGain], 10, 32)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to int: %v", contentsMap[audioInputGain], audioInputGain, err)
	}
	if inputGain < 0 || inputGain > 100 {
		s.Error("Failed. input_gain is not in a legal range [0, 100]: ", inputGain)
	}

	// Check "underruns" and "severe_underruns" are positive integer or zero.
	underruns, err := strconv.ParseInt(contentsMap[audioUnderruns], 10, 32)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to int: %v", contentsMap[audioUnderruns], audioUnderruns, err)
	}
	if underruns < 0 {
		s.Error("Failed. underruns is smaller than zero: ", underruns)
	}

	severeUnderruns, err := strconv.ParseInt(contentsMap[audioSevereUnderruns], 10, 32)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to int: %v", contentsMap[audioSevereUnderruns], audioSevereUnderruns, err)
	}
	if severeUnderruns < 0 {
		s.Error("Failed. severe_underruns is smaller than zero: ", severeUnderruns)
	}
}
