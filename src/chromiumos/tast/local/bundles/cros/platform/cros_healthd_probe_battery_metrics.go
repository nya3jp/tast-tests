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
		s.Fatal("Failed to run 'cros_healthd --probe_battery_metrics': ", err)
	}

	lines := strings.Split(string(b), "\n")
	if len(lines) != 2 {
		s.Fatal(errors.Errorf("Incorrect number of output lines. Want=2, Got=%d", len(lines)))
	}
	headerExpected := [7]string{"charge_full", "charge_full_design", "cycle_count", "serial_number", "vendor(manufacturer)", "voltage_now", "voltage_min_design"}
	headerActual := strings.Split(strings.TrimSpace(lines[0]), ",")
	if len(headerActual) != len(headerExpected) {
		s.Fatal(errors.Errorf("Incorrect header for battery metrics: %s", strings.TrimSpace(lines[0])))
	}
	for _, he := range headerExpected {
		found := true
		for _, ha := range headerActual {
			found = found || (ha == he)
		}
		if !found {
			s.Fatal(errors.Errorf("Missing battery metric: %s", he))
		}
	}
	metrics := strings.Split(strings.TrimSpace(lines[1]), ",")
	if len(metrics) != len(headerExpected) {
		s.Fatal(errors.Errorf("Incorrect number of battery metrics. Want=%d, Got=%d", len(headerExpected), len(metrics)))
	}
}
