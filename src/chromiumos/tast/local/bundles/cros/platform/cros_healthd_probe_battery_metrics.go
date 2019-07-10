// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	power "chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BatteryMetricsTest,
		Desc: "Check that we can probe cros_healthd for battery metrics",
		Contacts: []string{
			"wbbradley@google.com",
			"pmoy@google.com",
			"khegde@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func BatteryMetricsTest(ctx context.Context, s *testing.State) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		s.Fatal("Failed to get battery status: ", err)
	}
	b, err := testexec.CommandContext(ctx, "cros_healthd", "--probe_battery_metrics").Output(testexec.DumpLogOnError)
	if err != nil {
		// The command is expected to fail if there is no battery to retrive metrics for.
		if !status.BatteryPresent {
			return
		}
		s.Fatal("Failed to run 'cros_healthd --probe_battery_metrics': ", err)
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) != 2 {
		logFailureOutput(string(b))
		s.Fatalf("Incorrect number of output lines: got %d; want 2", len(lines))
	}
	want := sort.Slice([]string{"charge_full", "charge_full_design", "cycle_count", "serial_number", "vendor(manufacturer)", "voltage_now", "voltage_min_design"})
	got := sort.Slice(strings.Split(lines[0], ","))
	if got != want {
		s.Fatalf("header keys: got %v; want %v", got, want)
	}
	metrics := strings.Split(lines[1], ",")
	if len(metrics) != len(headerExpected) {
		logFailureOutput(string(b))
		s.Fatalf("Incorrect number of battery metrics: got %d; want %d", len(metrics), len(headerExpected))
	}
}

// If this test fails, it may be helpful to inspect the contents of the output.
func logFailureOutput(s *testing.State, output string) {
	path := filepath.Join(s.OutDir, "command_output.txt")
	if err := ioutil.WriteFile(path,
		[]byte(output), 0644); err != nil {
		s.Errorf("Failed to write output to %s: %v", path, err)
	}
}
