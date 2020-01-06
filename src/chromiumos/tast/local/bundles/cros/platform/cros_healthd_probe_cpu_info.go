// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeCPUInfo,
		Desc: "Check that we can probe cros_healthd for CPU info",
		Contacts: []string{
			"jschettler@google.com",
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeCPUInfo(ctx context.Context, s *testing.State) {
	b, err := testexec.CommandContext(ctx, "telem", "--category=cpu").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'telem --category=cpu': ", err)
	}

	// Every board should have at least one CPU. The output of the command is a
	// line of headers followed by a line of CPU info for each CPU. Verify at
	// least one line of CPU info exists.
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) < 2 {
		s.Fatal("Could not find any lines of CPU info in output: ", string(b))
	}

	// Verify the headers are correct.
	want := []string{"model_name", "architecture", "max_clock_speed_khz"}
	got := strings.Split(lines[0], ",")
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify each line of CPU info contains valid values.
	for _, line := range lines[1:] {
		s.Log("Checking line: ", line)
		vals := strings.Split(line, ",")
		if len(vals) != 3 {
			s.Errorf("Wrong number of values: got %v, want 3", len(vals))
			continue
		}

		if vals[0] == "" {
			s.Error("Empty model_name")
		}

		if vals[1] == "" {
			s.Error("Empty architecture")
		}

		i, err := strconv.Atoi(vals[2])
		if err != nil {
			s.Error("Failed to convert max_clock_speed_khz to integer: ", err)
		} else if i <= 0 {
			s.Error("Invalid max_clock_speed_khz")
		}
	}
}
