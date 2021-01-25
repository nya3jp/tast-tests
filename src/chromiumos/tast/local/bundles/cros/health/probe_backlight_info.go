// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"reflect"
	"strconv"

	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBacklightInfo,
		Desc: "Checks that cros_healthd can fetch backlight info",
		Contacts: []string{
			"jschettler@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeBacklightInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBacklight}
	records, err := croshealthd.RunAndParseTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get backlight telemetry info: ", err)
	}

	hasBacklight, err := crosconfig.Get(ctx, "/hardware-properties", "has-backlight")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to get has-backlight property: ", err)
	}

	if err == nil && hasBacklight == "false" {
		if len(records) != 1 {
			s.Fatalf("Incorrect number of ouput lines: got %d; want 1", len(records))
		}
		// If there is no backlight, there is no output to verify.
		return
	}

	if len(records) < 2 {
		s.Fatal("Could not find any lines of backlight info")
	}

	// Verify the headers are correct.
	want := []string{"path", "max_brightness", "brightness"}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify each line of backlight info contains valid values.
	for _, record := range records[1:] {
		if record[0] == "" {
			s.Error("Empty path")
		}

		maxBrightness, err := strconv.Atoi(record[1])
		if err != nil {
			s.Errorf("Failed to convert %q to integer: %v", want[1], err)
		} else if maxBrightness < 0 {
			s.Errorf("Invalid %s: %v", want[1], maxBrightness)
		}

		brightness, err := strconv.Atoi(record[2])
		if err != nil {
			s.Errorf("Failed to convert %q to integer: %v", want[2], err)
		} else if brightness < 0 {
			s.Errorf("Invalid %s: %v", want[2], brightness)
		}

		if brightness > maxBrightness {
			s.Errorf("brightness: %v greater than max_brightness: %v", brightness, maxBrightness)
		}
	}
}
