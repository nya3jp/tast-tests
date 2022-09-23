// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/local/power"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

const (
	batteryTraceConfigFile = "perfetto/battery_trace_cfg.pbtxt"
	batteryTraceQueryFile  = "perfetto/battery_counters.sql"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PerfettoBatteryDataSource,
		Desc:     "Verifies the linux.sysfs_power data source of traced_probes",
		Contacts: []string{"chinglinyu@chromium.org", "chenghaoyang@chromium.org"},
		Data: []string{batteryTraceConfigFile,
			batteryTraceQueryFile,
			tracing.TraceProcessorAmd64,
			tracing.TraceProcessorArm,
			tracing.TraceProcessorArm64},
		Attr: []string{"group:mainline", "informational"},
	})
}

// PerfettoBatteryDataSource checks that the "linux.sysfs_power" data source
// collects battery counters on the device.
func PerfettoBatteryDataSource(ctx context.Context, s *testing.State) {
	// Start a trace session using the perfetto command line tool.
	traceConfigPath := s.DataPath(batteryTraceConfigFile)
	sess, err := tracing.StartSessionAndWaitUntilDone(ctx, traceConfigPath)
	// The temporary file of trace data is no longer needed when returned.
	defer sess.RemoveTraceResultFile()

	if err != nil {
		s.Fatal("Failed to start tracing: ", err)
	}

	// Process the trace data with the SQL query and get [][]string as the result.
	// See the content of batteryTraceQueryFile for details.
	// Example result:
	// {
	//   { "name", "avg(value)" }
	//   { "batt.hid-0018:27C6:0E52.0001-battery.capacity_pct", "0.000000" }
	//   { "batt.sbs-12-000b.capacity_pct", "100.000000" }
	//   { "batt.sbs-12-000b.charge_uah", "5450000.000000" }
	//   { "batt.sbs-12-000b.current_ua", "0.000000" }
	// }
	batt, err := sess.RunQuery(ctx, s.DataPath(tracing.TraceProcessor()), s.DataPath(batteryTraceQueryFile))
	if err != nil {
		s.Fatal("Failed to process the trace data: ", err)
	}
	s.Log("Battery counters: ", batt)

	status, err := power.GetStatus(ctx)
	// Battery is not always available (e.g. on VM). Skip validation if the device is equipped with a battery.
	if !status.BatteryPresent {
		s.Log("Skipped validation of battery counters: battery is not present")
		return
	}

	var capacity, charge, current []float64 // Use slices since there can be multiple batteries.
	for _, row := range batt[1:] {          // Skip the 1st row of column names.
		name, val := row[0], row[1]
		v, err := strconv.ParseFloat(val, 64)
		if err != nil {
			s.Fatalf("Invalid battery counter: %s: %s", name, val)
		}
		if strings.HasSuffix(name, "capacity_pct") {
			capacity = append(capacity, v)
		} else if strings.HasSuffix(name, "charge_uah") {
			charge = append(charge, v)
		} else if strings.HasSuffix(name, "current_ua") {
			current = append(current, v)
		} else {
			s.Fatalf("Unexpected battery counter: %s", name)
		}
	}

	validateValueRange := func(vals []float64, lower, upper float64) bool {
		if vals == nil {
			return false
		}
		for _, v := range vals {
			if v > upper || v < lower {
				return false
			}
		}
		return true
	}

	if status.BatteryPercent != 0.0 {
		if !validateValueRange(capacity, 0.0, 100.0) {
			s.Fatal("Invalid battery capacity value: ", capacity)
		}
	}
	// 100 Ah is a huge battery that we should not have a device with this large battery.
	const maxBatteryChargeUAH = 100 * 1e6
	// Note that status.BatteryCharge is in Ah, while charge is in uAh.
	if status.BatteryCharge != 0.0 {
		if !validateValueRange(charge, 0.0, maxBatteryChargeUAH) {
			s.Fatal("Invalid battery charge value: ", charge)
		}
	}
	// Note that status.BatteryCurrent is in A, while current is in uA.
	if status.BatteryCurrent != 0.0 {
		// Don't assert the value of current since it can be positive or negative.
		// The kernel doc states that for batteries, negative values are used for discharge, but not all drivers follow that.
		if current == nil {
			s.Fatal("Battery current counter is missing")
		}
	}
}
