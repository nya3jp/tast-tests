// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdBatteryMetricsProbe,
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

func CrosHealthdProbeBatteryMetrics(ctx context.Context, s *testing.State) {
	b, err := testexec.CommandContext(ctx, "cros_healthd", "--probe_battery_metrics").Output(testexec.DumpLogOnError)
	if err != nil {
		filename := "/tmp/cros_healthd_battery_probe"
		if filenameErr = ioutil.WriteFile(filename, []byte(b), 0644); filenameErr != nil {
			s.Error("Failed to write output of 'cros_healthd --probe_battery_metrics': ", filenameErr)
		}
		s.Fatal("Failed to run 'cros_healthd --probe_battery_metrics': ", err)
	}

	lines := strings.Split(string(b), "\n")
	if len(lines) != 2 {
		s.Fatalf("Incorrect number of output lines: got %d; want 2", len(lines))
	}
	headerExpected := []string{"charge_full", "charge_full_design", "cycle_count", "serial_number", "vendor(manufacturer)", "voltage_now", "voltage_min_design"}
	headerActualMap := map[string]struct{}{}
	for _, actualKey := range strings.Split(lines[0], ",") {
		headerActualMap[actualKey] = struct{}{}
	}
	missing := []string{}
	for _, h := range headerExpected {
		if _, ok = headerActualMap[h]; !ok {
			missing = append(missing, h)
			continue
		}
		delete(headerActualMap, h)
	}
	// This is the case where expected elements are missing.
	if len(missing) > 0 {
		s.Fatalf("Missing keys: %s", strings.Join(missing, ","))
	}
	// This is the case where there are actually more elements that there should be.
	if len(headerActualMap) > 0 {
		extra := []string{}
		for k := range headerActualMap {
			extra = append(extra, k)
		}
		s.Fatalf("Found unexpected keys: %s", strings.Join(extra, ","))
	}
	metrics := strings.Split(lines[1], ",")
	if len(metrics) != len(headerExpected) {
		s.Fatalf("Incorrect number of battery metrics: got=%d, want=%d", len(metrics), len(headerExpected))
	}
}
