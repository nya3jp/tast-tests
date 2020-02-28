// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeBacklightInfo,
		Desc: "Check that we can probe cros_healthd for backlight info",
		Contacts: []string{
			"jschettler@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_config", "diagnostics"},
	})
}

func CrosHealthdProbeBacklightInfo(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start cros_healthd: ", err)
	}

	b, err := testexec.CommandContext(ctx, "telem", "--category=backlight").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Command failed: ", err)
	}

	// Log output to file for debugging.
	path := filepath.Join(s.OutDir(), "command_output.txt")
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		s.Errorf("Failed to write output to %s: %v", path, err)
	}

	val, err := crosconfig.Get(ctx, "/cros-healthd/backlight", "has-backlight")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to get has-backlight property: ", err)
	}

	hasBacklight := !(err == nil && val == "false")
	s.Log("Device has backlight: ", hasBacklight)

	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if !hasBacklight {
		if len(lines) != 1 {
			s.Fatalf("Incorrect number of output lines: got %d; want 1", len(lines))
		}
		// If there is no backlight, there is no output to verify.
		return
	}

	if len(lines) < 2 {
		s.Fatal("Could not find any lines of backlight info")
	}

	// Verify the headers are correct.
	want := []string{"path", "max_brightness", "brightness"}
	got := strings.Split(lines[0], ",")
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify each line of backlight info contains valid values.
	for _, line := range lines[1:] {
		s.Log("Checking line: ", line)
		vals := strings.Split(line, ",")
		if len(vals) != 3 {
			s.Errorf("Wrong number of values: got %v, want 3", len(vals))
			continue
		}

		if vals[0] == "" {
			s.Error("Empty path")
		}

		maxBrightness, err := strconv.Atoi(vals[1])
		if err != nil {
			s.Errorf("Failed to convert %q to integer: %v", want[1], err)
		} else if maxBrightness < 0 {
			s.Errorf("Invalid %s: %v", want[1], maxBrightness)
		}

		brightness, err := strconv.Atoi(vals[2])
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
