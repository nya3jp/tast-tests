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
	"syscall"

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

func absDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
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

	if len(lines) < 2 {
		s.Fatalf("Wrong number of output lines: got %v; want 2", len(lines))
	}

	// Verify the headers are correct.
	want := []string{"available_space", "total_space"}
	got := lines[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify the values are correct.
	vals := lines[1]
	if len(vals) != 2 {
		s.Fatalf("Wrong number of values: got %v; want 2", len(vals))
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/mnt/stateful_partition", &stat); err != nil {
		s.Fatalf("Failed to get disk stats for %s: %v", path, err)
	}

	realAvailable := stat.Bavail * uint64(stat.Bsize)
	margin := uint64(100000000) // 100MB
	realTotal := stat.Blocks * uint64(stat.Bsize)

	if available, err := strconv.ParseUint(vals[0], 10, 64); err != nil {
		s.Errorf("Failed to convert %q (available_space) to uint64: %v", vals[0], err)
	} else if absDiff(available, realAvailable) > margin {
		s.Errorf("Invalid available_space: got %v; want %v +- %v", available, realAvailable, margin)
	}

	if total, err := strconv.ParseUint(vals[1], 10, 64); err != nil {
		s.Errorf("Failed to convert %q (total_space) to uint64: %v", vals[1], err)
	} else if total != realTotal {
		s.Errorf("Invalid total_space: got %v; want %v", total, realTotal)
	}
}
