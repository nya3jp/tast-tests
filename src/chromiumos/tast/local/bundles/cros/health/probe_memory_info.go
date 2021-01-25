// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeMemoryInfo,
		Desc: "Check that we can probe cros_healthd for memory info",
		Contacts: []string{
			"jschettler@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeMemoryInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryMemory}
	records, err := croshealthd.RunAndParseTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get memory telemetry info: ", err)
	}

	if len(records) != 2 {
		s.Fatalf("Wrong number of output lines: got %d; want 2", len(records))
	}

	want := []string{"total_memory_kib", "free_memory_kib", "available_memory_kib",
		"page_faults_since_last_boot"}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Each memory metric should be a positive integer. This assumes that all
	// machines will always have at least 1 free KiB of memory, and all machines
	// will have page faulted at least once between boot and the time this test
	// finishes executing.
	for i, metric := range records[1] {
		if val, err := strconv.Atoi(metric); err != nil {
			s.Errorf("Failed to convert %q to integer: %v", metric, err)
		} else if val <= 0 {
			s.Errorf("Invalid %s: %v", want[i], val)
		}
	}
}
