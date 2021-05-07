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
	audioUserMute     = "user_mute"
	audioCaptureMute  = "capture_mute"
	audioActiveNode   = "active_node"
	audioActiveVolume = "active_volume"
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
	want := []string{audioUserMute, audioCaptureMute, audioActiveNode, audioActiveVolume}
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

	// Check "user_mute" and "capture_mute" is bool
	if _, err := strconv.ParseBool(contentsMap[audioUserMute]); err != nil {
		s.Errorf("Failed to convert %q (user_mute) to bool: %v", contentsMap[audioUserMute], err)
	}
	if _, err := strconv.ParseBool(contentsMap[audioCaptureMute]); err != nil {
		s.Errorf("Failed to convert %q (capture_mute) to bool: %v", contentsMap[audioCaptureMute], err)
	}

	// Check "active_node" is not empty
	if contentsMap[audioActiveNode] == "" {
		s.Error("Failed. active_node field is empty")
	}

	// Check "active_volume" is integer and between [0, 100]
	activeVolume, err := strconv.ParseInt(contentsMap[audioActiveVolume], 10, 32)
	if err != nil {
		s.Errorf("Failed to convert %q (active_volume) to int: %v", contentsMap[audioActiveVolume], err)
	}
	if activeVolume < 0 || activeVolume > 100 {
		s.Errorf("Failed. active_volume is not in a legal range [0, 100]: %d", activeVolume)
	}
}
