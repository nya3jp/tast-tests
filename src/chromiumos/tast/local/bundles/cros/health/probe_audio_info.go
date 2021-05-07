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
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeAudioInfo,
		Desc: "Check that we can probe cros_healthd for audio info",
		Contacts: []string{
			"kerker@google.com",
			"cros-tdm@google.com",
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
	want := []string{audioOutputMute, audioInputMute, audioOutputDeviceName, audioOutputVolume}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify the values are correct.
	vals := records[1]
	if len(vals) != len(want) {
		s.Fatalf("Wrong number of values: got %d; want %d", len(vals), len(want))
	}

	// Using a map, then we don't need to take care the index change in future.
	contentsMap := make(map[string]string)
	for i, elem := range want {
		contentsMap[elem] = vals[i]
	}

	// Check "output_mute" and "input_mute" is bool
	if _, err := strconv.ParseBool(contentsMap[audioOutputMute]); err != nil {
		s.Errorf("Failed to convert %q (output_mute) to bool: %v", contentsMap[audioOutputMute], err)
	}
	if _, err := strconv.ParseBool(contentsMap[audioInputMute]); err != nil {
		s.Errorf("Failed to convert %q (input_mute) to bool: %v", contentsMap[audioInputMute], err)
	}

	// Check "output_device_name" is not empty
	if contentsMap[audioOutputDeviceName] == "" {
		s.Error("Failed. output_device_name field is empty")
	}

	// Check "output_volume" is integer and between [0, 100]
	outputVolume, err := strconv.ParseInt(contentsMap[audioOutputVolume], 10, 32)
	if err != nil {
		s.Errorf("Failed to convert %q (output_volume) to int: %v", contentsMap[audioOutputVolume], err)
	}
	if outputVolume < 0 || outputVolume > 100 {
		s.Errorf("Failed. output_volume is not in a legal range [0, 100]: %d", outputVolume)
	}
}
