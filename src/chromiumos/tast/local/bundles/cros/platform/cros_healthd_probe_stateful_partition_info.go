// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/csv"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeStatefulPartitionInfo,
		Desc: "Checks that cros_healthd can fetch stateful partition info",
		Contacts: []string{
			"jschettler@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeStatefulPartitionInfo(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start cros_healthd: ", err)
	}

	b, err := testexec.CommandContext(ctx, "telem", "--category=stateful_partition").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Command failed: ", err)
	}

	// Log output to file for debugging.
	path := filepath.Join(s.OutDir(), "command_output.txt")
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		s.Errorf("Failed to write output to %s: %v", path, err)
	}

	lines, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		s.Fatal("Failed to parse output: ", err)
	}

	if len(lines) < 1 {
		s.Fatal("Output contains no lines")
	}

	// Verify the headers are correct.
	want := []string{"available_space", "total_space"}
	got := lines[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	if len(lines) < 2 {
		s.Fatal("Output contains no values")
	}

	// Verify the values are valid.
	vals := lines[1]
	if len(vals) != 2 {
		s.Fatalf("Wrong number of values: got %v, want 2", len(vals))
	}

	if available, err := strconv.Atoi(vals[0]); err != nil {
		s.Errorf("Failed to convert %q (%s) to integer: %v", vals[0], want[0], err)
	} else if available < 0 {
		s.Errorf("Invalid %s (cannot be less than 0): %v", want[0], available)
	}

	if total, err := strconv.Atoi(vals[1]); err != nil {
		s.Errorf("Failed to convert %q (%s) to integer: %v", vals[1], want[1], err)
	} else if total < 0 {
		s.Errorf("Invalid %s (cannot be less than 0): %v", want[1], total)
	}
}
