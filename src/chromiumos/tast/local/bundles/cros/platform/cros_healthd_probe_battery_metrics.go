// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"reflect"

	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/croshealthd"
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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"cros_config", "diagnostics"},
	})
}

func CrosHealthdProbeBatteryMetrics(ctx context.Context, s *testing.State) {
	records, err := croshealthd.FetchTelemetry(ctx, croshealthd.TelemCategoryBattery, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get battery telemetry info: ", err)
	}

	val, err := crosconfig.Get(ctx, "/hardware-properties", "psu-type")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to get psu-type property: ", err)
	}

	// If psu-type is not set to "AC_only", assume there is a battery.
	hasBattery := !(err == nil && val == "AC_only")
	if !hasBattery {
		if len(records) != 1 {
			s.Fatalf("Incorrect number of records: got %d; want 1", len(records))
		}
		// If there is no battery, there are no records to verify.
		return
	}

	if len(records) != 2 {
		s.Fatalf("Incorrect number of records: got %d; want 2", len(records))
	}

	want := []string{"charge_full", "charge_full_design", "cycle_count",
		"serial_number", "vendor(manufacturer)", "voltage_now",
		"voltage_min_design", "manufacture_date_smart", "temperature_smart",
		"model_name", "charge_now", "current_now", "technology", "status"}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v, want %v", got, want)
	}

	metrics := records[1]
	if len(metrics) != len(want) {
		s.Fatalf("Incorrect number of battery metrics: got %d; want %d", len(metrics), len(want))
	}

	// Validate battery metrics.
	contentsMap := make(map[string]string)
	for i, elem := range got {
		contentsMap[elem] = metrics[i]
	}
	for _, key := range []string{"charge_full", "charge_full_design",
		"cycle_count", "serial_number", "vendor(manufacturer)", "voltage_now",
		"voltage_min_design", "model_name", "charge_now", "current_now",
		"technology", "status"} {
		value, ok := contentsMap[key]
		if !ok {
			s.Errorf("Key %q not found", key)
			continue
		}

		s.Logf("Value for %v: %v", key, value)
		if value == "" {
			s.Error("Missing ", key)
		}
	}

	// Validate Smart Battery metrics.
	val, err = crosconfig.Get(ctx, "/cros-healthd/battery", "has-smart-battery-info")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to get has-smart-battery-info property: ", err)
	}

	hasSmartInfo := err == nil && val == "true"
	s.Log("Device has Smart Battery info: ", hasSmartInfo)
	for _, e := range []struct {
		key     string
		zeroFmt string
	}{
		{key: "manufacture_date_smart", zeroFmt: "0000-00-00"},
		{key: "temperature_smart", zeroFmt: "0"},
	} {
		value, ok := contentsMap[e.key]
		if !ok {
			s.Errorf("Key %q not found", e.key)
			continue
		}

		s.Logf("Value for %v: %v", e.key, value)
		if hasSmartInfo {
			if value == "NA" || value == e.zeroFmt {
				s.Error("Invalid value for ", e.key)
			}
		} else {
			if value != "NA" {
				s.Errorf("Value for %v should be NA", e.key)
			}
		}
	}
}
