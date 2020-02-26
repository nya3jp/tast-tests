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

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeMemoryInfo,
		Desc: "Check that we can probe cros_healthd for memory info",
		Contacts: []string{
			"jschettler@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeMemoryInfo(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start cros_healthd: ", err)
	}

	b, err := testexec.CommandContext(ctx, "telem", "--category=memory").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Command failed: ", err)
	}

	// Log output to file for debugging.
	path := filepath.Join(s.OutDir(), "command_output.txt")
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		s.Errorf("Failed to write output to %s: %v", path, err)
	}

	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 2 {
		s.Fatalf("Wrong number of lines: got %v, want 2", len(lines))
	}

	want := []string{"total_memory_kib", "free_memory_kib", "available_memory_kib",
		"page_faults_since_last_boot"}
	got := strings.Split(lines[0], ",")
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v, want %v", got, want)
	}

	metrics := strings.Split(lines[1], ",")
	if len(metrics) != len(want) {
		s.Fatalf("Incorrect number of memory metrics: got %d; want %d", len(metrics), len(want))
	}

	// Each memory metric should be a positive integer. This assumes that all
	// machines will always have at least 1 free KiB of memory, and all machines
	// will have page faulted at least once between boot and the time this test
	// finishes executing.
	for i := 0; i < len(metrics); i++ {
		s.Log("Checking value for ", want[i])
		val, err := strconv.Atoi(metrics[i])
		if err != nil {
			s.Errorf("Failed to convert %q to integer: %v", metrics[i], err)
		} else if val <= 0 {
			s.Errorf("Invalid %s: %v", want[i], val)
		}
	}
}
