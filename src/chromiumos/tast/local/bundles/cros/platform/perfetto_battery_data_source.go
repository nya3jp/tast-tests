// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strconv"

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
		Data:     []string{batteryTraceConfigFile, batteryTraceQueryFile, tracing.TraceProcessor()},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// PerfettoBatteryDataSource checks that the "linux.sysfs_power" data source
// collects battery counters on the device.
func PerfettoBatteryDataSource(ctx context.Context, s *testing.State) {
	// Start a trace session using the perfetto command line tool.
	traceConfigPath := s.DataPath(batteryTraceConfigFile)
	s.Log(traceConfigPath)
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
	//   { "capacity_percent", "charge_uah", "current_ua" }
	//   { "100.000000", "3005000.000000", "18454.545455" }
	// }
	batt, err := sess.RunQuery(ctx, s.DataPath(tracing.TraceProcessor()), s.DataPath(batteryTraceQueryFile))
	if err != nil {
		s.Fatal("Failed to process the trace data: ", err)
	}
	s.Log("Battery counters: ", batt)

	names := batt[0]
	if names[0] != "capacity_percent" || names[1] != "charge_uah" || names[2] != "current_ua" {
		s.Fatal("Unexpected query column names: ", names)
	}

	status, err := power.GetStatus(ctx)
	// Battery is not always available (e.g. on VM). Skip validation if the device is equipped with a battery.
	if !status.BatteryPresent {
		s.Log("Skipped validation of battery counters: battery is not present")
		return
	}

	capacity, charge, current := batt[1][0], batt[1][1], batt[1][2]
	if status.BatteryPercent != 0.0 {
		c, err := strconv.ParseFloat(capacity, 64)
		if err != nil || c < 0.0 || c > 100.0 {
			s.Fatalf("Invalid battery capacity value: %s", capacity)
		}

	}
	// Note that status.BatteryCharge is in Ah, while charge is in mAh.
	if status.BatteryCharge != 0.0 {
		c, err := strconv.ParseFloat(charge, 64)
		if err != nil || c < 0.0 {
			s.Fatalf("Invalid battery charge value: %s", charge)
		}
	}
	// Note that status.BatteryCurrent is in A, while current is in mA.
	if status.BatteryCurrent != 0.0 {
		_, err = strconv.ParseFloat(current, 64)
		// Don't assert the value of current since it can be positive or negative.
		// The kernel doc states that for batteries, negative values are used for discharge, but not all drivers follow that.
		if err != nil {
			s.Fatalf("Invalid battery current value: %s", current)
		}
	}
}
