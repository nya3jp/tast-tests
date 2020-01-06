// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeBatteryMetrics,
		Desc: "Check that we can probe cros_healthd for battery metrics",
		Contacts: []string{
			"pmoy@google.com",
			"khegde@google.com",
			"jschettler@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeBatteryMetrics(ctx context.Context, s *testing.State) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		s.Fatal("Failed to get battery status: ", err)
	}
	b, err := testexec.CommandContext(ctx, "telem", "--category=battery").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'telem --category=battery': ", err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "command_output.txt"), b, 0644); err != nil {
		s.Errorf("Failed to write output to %s: %v", filepath.Join(s.OutDir(), "command_output.txt"), err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if status.BatteryPresent && len(lines) != 2 {
		s.Fatalf("Incorrect number of output lines: got %d; want 2", len(lines))
	}
	want := []string{"charge_full", "charge_full_design", "cycle_count", "serial_number", "vendor(manufacturer)", "voltage_now", "voltage_min_design", "manufacture_date_smart", "temperature_smart", "model_name", "charge_now"}
	sort.Strings(want)
	header := strings.Split(lines[0], ",")
	got := make([]string, len(header))
	copy(got, header)
	sort.Strings(got)
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("header keys: got %v; want %v", got, want)
	}
	if len(lines) == 2 {
		metrics := strings.Split(lines[1], ",")
		if len(metrics) != len(want) {
			s.Fatalf("Incorrect number of battery metrics: got %d; want %d", len(metrics), len(want))
		}
		// Validate smart battery metrics
		contentsMap := make(map[string]string)
		for i, elem := range header {
			contentsMap[elem] = metrics[i]
		}
		if contentsMap["manufacture_date_smart"] == "0" && contentsMap["temperature_smart"] == "0" {
			s.Fatal("Failed to collect manufacture_date_smart and temperature_smart")
		}
		if contentsMap["manufacture_date_smart"] == "0" {
			s.Fatal("Failed to collect manufacture_date_smart")
		}
		if contentsMap["temperature_smart"] == "0" {
			s.Fatal("Failed to collect temperature_smart")
		}
	}
}
